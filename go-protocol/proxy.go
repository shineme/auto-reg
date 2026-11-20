package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/proxy"
)

// ProxyStyle 区分不同代理商的用户名参数体系。
type ProxyStyle int

const (
	// StyleAuto 按主机名自动识别（cliproxy.io / 711proxy.com / trustsource.live）
	StyleAuto ProxyStyle = iota
	// Style711 使用 711 参数：session-<随机> 与 sessTime-<分钟>
	Style711
	// StyleClipProxy 使用 Cliproxy 参数：sid-<随机> 与 t-<分钟>，并支持 st-<州省>
	StyleClipProxy
)

type ProxyDialer struct {
	rawURL   string
	parsed   *url.URL
	socks    proxy.Dialer
	mu       sync.Mutex
	session  string
	// 节点池模式：每个节点一个出口 IP，RotateSession 时轮换到下一个节点。
	nodes    []proxyNodeSpec
	nodeIdx  int
	// 代理商参数：区域轮换、sticky 会话时长（分钟）、州省、风格、是否 sticky
	regions   []string
	sessTime  int
	state     string
	style     ProxyStyle
	sticky    bool
	// rotateSession=true 时忽略用户名里写死的会话标识，每次轮换都换新 IP。
	rotateSession bool
	regionIdx int
}

// proxyNodeSpec 描述节点池里的一个直连节点。
type proxyNodeSpec struct {
	Scheme string // http / https / socks5
	Host   string // host:port
	User   string
	Pass   string
	SNI    string
	Name   string
}

var (
	// regionRe 匹配 region-XX（国家码）/ region-Rand（商家随机国家）。
	regionRe = regexp.MustCompile(`(?i)-region-[A-Za-z]{2,12}`)
	// Cliproxy 的会话标识参数：sid-<值>；711 的等价参数是 session-<值>
	sidRe     = regexp.MustCompile(`(?i)(-sid-[^-]+)`)
	sessionRe = regexp.MustCompile(`(?i)(-session-[^-]+)`)
	// Cliproxy 的 IP 持续时间参数：t-<分钟>
	clipTRe = regexp.MustCompile(`(?i)(-t-\d+)`)
	// 711 的 IP 持续时间参数：sessTime-<分钟>
	sessTimeRe = regexp.MustCompile(`(?i)(-sessTime-\d+)`)
	// Cliproxy 州省参数：st-<州省>
	stateRe = regexp.MustCompile(`(?i)(-st-[^-]+)`)
)

// detectProxyStyle 依据主机名猜测代理商的参数体系。
func detectProxyStyle(host string) ProxyStyle {
	h := strings.ToLower(host)
	switch {
	case strings.Contains(h, "cliproxy"), strings.Contains(h, "zooproxy"):
		return StyleClipProxy
	case strings.Contains(h, "711proxy"), strings.Contains(h, "trustsource"):
		return Style711
	default:
		return StyleAuto
	}
}

// SetOptions 配置代理商参数：
// regions 非空时每次轮换区域（如 ["JP","US"]）；sessTime 控制 sticky 时长；
// style 指定参数体系（0=自动识别）；state 为 Cliproxy 的 st-<州省>；
// sticky=false 时不追加会话参数，即 Rotating 模式（每请求换 IP）。
func (p *ProxyDialer) SetOptions(regions []string, sessTime int, style ProxyStyle, state string, sticky bool) {
	p.SetOptionsEx(regions, sessTime, style, state, sticky, false)
}

// SetOptionsEx 在 SetOptions 基础上增加 rotateSession：
// 为 true 时忽略用户名里写死的 session/sid，每次轮换都生成新会话（新出口 IP）。
func (p *ProxyDialer) SetOptionsEx(regions []string, sessTime int, style ProxyStyle, state string, sticky bool, rotateSession bool) {
	if p == nil {
		return
	}
	p.regions = regions
	if sessTime < 5 {
		sessTime = 0
	}
	if sessTime > 180 {
		sessTime = 180
	}
	p.sessTime = sessTime
	p.style = style
	p.state = strings.TrimSpace(state)
	p.sticky = sticky
	p.rotateSession = rotateSession
}

// SetPoolNodes 切换到节点池模式：每次 RotateSession 轮换到下一个节点出口。
// offset 用于让不同线程从不同节点起步，避免并发线程撞同一个出口。
func (p *ProxyDialer) SetPoolNodes(nodes []proxyNodeSpec, offset int) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.nodes = nodes
	if offset != 0 && len(nodes) > 0 {
		p.nodeIdx = ((offset % len(nodes)) + len(nodes)) % len(nodes)
	} else {
		p.nodeIdx = 0
	}
}

// PoolSize 返回当前节点池大小。
func (p *ProxyDialer) PoolSize() int {
	if p == nil {
		return 0
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.nodes)
}

// CurrentNodeName 返回当前使用的节点名称（便于日志定位出口）。
func (p *ProxyDialer) CurrentNodeName() string {
	if p == nil {
		return ""
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.nodes) == 0 {
		return ""
	}
	return p.nodes[p.nodeIdx%len(p.nodes)].Name
}

// rotateRegion 在用户名上替换或追加 711 的 region 参数。
func rotateRegion(base string, region string) string {
	if region == "" {
		return base
	}
	if m := regionRe.FindString(base); m != "" {
		return regionRe.ReplaceAllString(base, "-region-"+region)
	}
	return base + "-region-" + region
}

// atoiSafe 解析数字，失败返回 0。
func atoiSafe(s string) int {
	n := 0
	for _, r := range strings.TrimSpace(s) {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
	}
	return n
}

func NewProxyDialer(raw string) (*ProxyDialer, error) {
	norm, err := NormalizeProxyURL(raw)
	if err != nil {
		return nil, err
	}
	if norm == "" {
		return &ProxyDialer{}, nil
	}
	u, err := url.Parse(norm)
	if err != nil {
		return nil, err
	}
	pd := &ProxyDialer{rawURL: norm, parsed: u}
	pd.RotateSession()
	return pd, nil
}

func (p *ProxyDialer) Enabled() bool {
	if p == nil {
		return false
	}
	if p.parsed != nil {
		return true
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.nodes) > 0
}

func (p *ProxyDialer) RotateSession() {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	// 节点池模式：直接轮换到下一个节点，每个节点一个出口 IP。
	if len(p.nodes) > 0 {
		p.nodeIdx = (p.nodeIdx + 1) % len(p.nodes)
		p.session = ""
		return
	}
	if p.parsed == nil || p.parsed.User == nil {
		return
	}
	base := strings.TrimSpace(p.parsed.User.Username())
	if base == "" {
		return
	}

	style := p.style
	if style == StyleAuto {
		style = detectProxyStyle(p.parsed.Hostname())
	}

	// 用户名里已经写死了会话标识（711: -session-/-sessid-，Cliproxy: -sid-）：
	// 尊重用户设置，不自动轮换。
	lower := strings.ToLower(base)
	hasFixedSession := strings.Contains(lower, "-session-") || strings.Contains(lower, "_session-") ||
		strings.Contains(lower, "-sessid-") || sidRe.MatchString(base)
	if hasFixedSession && !p.rotateSession {
		p.session = base
		return
	}
	if hasFixedSession {
		// 强制换会话：把写死的 sid / session 去掉，下面重新生成。
		base = sidRe.ReplaceAllString(base, "")
		base = sessionRe.ReplaceAllString(base, "")
		// 原来写死的时长参数先摘出来（没配 sessTime 时沿用它）。
		if m := clipTRe.FindString(base); m != "" && p.sessTime <= 0 {
			p.sessTime = atoiSafe(strings.TrimPrefix(m, "-t-"))
		}
		if m := sessTimeRe.FindString(base); m != "" && p.sessTime <= 0 {
			p.sessTime = atoiSafe(strings.TrimPrefix(m, "-sessTime-"))
		}
		base = clipTRe.ReplaceAllString(base, "")
		base = sessTimeRe.ReplaceAllString(base, "")
		base = strings.TrimSuffix(strings.TrimSpace(base), "-")
	}

	// 区域轮换（711 与 Cliproxy 参数名一致，都是 region-XX）
	if len(p.regions) > 0 {
		p.regionIdx = (p.regionIdx + 1) % len(p.regions)
		base = rotateRegion(base, p.regions[p.regionIdx])
	}

	// Rotating 模式：不带会话参数，每个请求换 IP
	if !p.sticky {
		p.session = base
		return
	}

	suffix := fmt.Sprintf("%d%d", time.Now().UnixNano(), time.Now().Unix()%997)
	switch style {
	case StyleClipProxy:
		// Cliproxy: sid-<唯一标识>，改 sid 即换 IP；t-<分钟> 为 IP 持续时间
		if p.state != "" {
			base = applyState(base, p.state)
		}
		sid := randomAlnum(8)
		session := base + "-sid-" + sid
		if p.sessTime > 0 {
			session += fmt.Sprintf("-t-%d", p.sessTime)
		}
		p.session = session
	default:
		// 711: session-<随机>，sessTime-<分钟>
		session := base + "-session-" + suffix
		if p.sessTime > 0 {
			session += fmt.Sprintf("-sessTime-%d", p.sessTime)
		}
		p.session = session
	}
}

// applyState 设置/替换 Cliproxy 的 st-<州省> 参数。
func applyState(base, state string) string {
	state = strings.TrimSpace(state)
	if state == "" {
		return base
	}
	if stateRe.MatchString(base) {
		return stateRe.ReplaceAllString(base, "-st-"+state)
	}
	return base + "-st-" + state
}

func (p *ProxyDialer) stickyUser() string {
	if p == nil || p.parsed == nil || p.parsed.User == nil {
		return ""
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.session == "" {
		return p.parsed.User.Username()
	}
	return p.session
}

// proxyTarget 描述当前拨号目标（节点池模式取当前节点，否则取基础地址 + 会话用户）。
type proxyTarget struct {
	scheme string
	host   string
	user   string
	pass   string
	sni    string
}

func (p *ProxyDialer) currentTarget() proxyTarget {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.nodes) > 0 {
		n := p.nodes[p.nodeIdx%len(p.nodes)]
		return proxyTarget{scheme: n.Scheme, host: n.Host, user: n.User, pass: n.Pass, sni: n.SNI}
	}
	t := proxyTarget{scheme: "http"}
	if p.parsed != nil {
		t.scheme = strings.ToLower(p.parsed.Scheme)
		t.host = p.parsed.Host
		if p.parsed.User != nil {
			t.user = p.parsed.User.Username()
			t.pass, _ = p.parsed.User.Password()
		}
	}
	if p.session != "" {
		t.user = p.session
	}
	return t
}

func (p *ProxyDialer) auth() *proxy.Auth {
	t := p.currentTarget()
	if t.user == "" {
		return nil
	}
	return &proxy.Auth{User: t.user, Password: t.pass}
}

func (p *ProxyDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	if p == nil {
		return (&net.Dialer{Timeout: 15 * time.Second}).DialContext(ctx, network, addr)
	}
	t := p.currentTarget()
	if t.host == "" {
		return (&net.Dialer{Timeout: 15 * time.Second}).DialContext(ctx, network, addr)
	}
	if strings.HasPrefix(t.scheme, "socks5") {
		var lastErr error
		for attempt := 0; attempt < 3; attempt++ {
			auth := p.auth()
			d, err := proxy.SOCKS5("tcp", t.host, auth, &net.Dialer{Timeout: 10 * time.Second})
			if err != nil {
				return nil, fmt.Errorf("初始化 SOCKS5 拨号器失败: %w", err)
			}
			type dialRes struct {
				conn net.Conn
				err  error
			}
			ch := make(chan dialRes, 1)
			go func() {
				c, err := d.Dial(network, addr)
				ch <- dialRes{c, err}
			}()
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case res := <-ch:
				if res.err == nil {
					return res.conn, nil
				}
				lastErr = res.err
			}
			time.Sleep(250 * time.Millisecond)
		}
		return nil, lastErr
	}
	// HTTP/HTTPS CONNECT tunnel
	var conn net.Conn
	var err error
	if strings.EqualFold(t.scheme, "https") {
		sni := t.sni
		if sni == "" {
			sni = hostOnly(t.host)
		}
		conn, err = tls.DialWithDialer(&net.Dialer{Timeout: 15 * time.Second}, "tcp", t.host,
			&tls.Config{InsecureSkipVerify: true, ServerName: sni})
	} else {
		conn, err = (&net.Dialer{Timeout: 15 * time.Second}).DialContext(ctx, "tcp", t.host)
	}
	if err != nil {
		return nil, err
	}
	req := fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\n", addr, addr)
	if t.user != "" {
		cred := base64.StdEncoding.EncodeToString([]byte(t.user + ":" + t.pass))
		req += "Proxy-Authorization: Basic " + cred + "\r\n"
	}
	req += "\r\n"
	if _, err := conn.Write([]byte(req)); err != nil {
		_ = conn.Close()
		return nil, err
	}
	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, &http.Request{Method: http.MethodConnect})
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		_ = conn.Close()
		return nil, fmt.Errorf("HTTP CONNECT 代理返回 %d", resp.StatusCode)
	}
	return conn, nil
}

// hostOnly 从 host:port 中取出主机名。
func hostOnly(hostport string) string {
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		return h
	}
	return hostport
}

func (p *ProxyDialer) HTTPClient(timeout time.Duration) *http.Client {
	tr := &http.Transport{
		DialContext:           p.DialContext,
		TLSHandshakeTimeout:   15 * time.Second,
		ResponseHeaderTimeout: timeout,
	}
	return &http.Client{Transport: tr, Timeout: timeout}
}

// HTTPClientViaBridge forces all HTTPS requests through a local unauthenticated HTTP CONNECT bridge.
// Chrome and protocol traffic then share the exact same sticky upstream session.
func (p *ProxyDialer) HTTPClientViaBridge(bridgeURL string, timeout time.Duration) *http.Client {
	if strings.TrimSpace(bridgeURL) == "" {
		return p.HTTPClient(timeout)
	}
	proxyURL, err := url.Parse(bridgeURL)
	if err != nil {
		return p.HTTPClient(timeout)
	}
	tr := &http.Transport{
		Proxy:                 http.ProxyURL(proxyURL),
		TLSHandshakeTimeout:   15 * time.Second,
		ResponseHeaderTimeout: timeout,
	}
	return &http.Client{Transport: tr, Timeout: timeout}
}

// LocalHTTPBridge exposes a local unauthenticated HTTP proxy for Chrome CDP when upstream proxy requires auth/socks5.
type LocalHTTPBridge struct {
	listener net.Listener
	dialer   *ProxyDialer
	mu       sync.Mutex
	closed   bool
}

func StartLocalHTTPBridge(upstreamRaw string) (*LocalHTTPBridge, string, error) {
	pd, err := NewProxyDialer(upstreamRaw)
	if err != nil {
		return nil, "", err
	}
	if !pd.Enabled() {
		return nil, "", nil
	}
	return StartLocalHTTPBridgeWithDialer(pd)
}

func StartLocalHTTPBridgeWithDialer(pd *ProxyDialer) (*LocalHTTPBridge, string, error) {
	if pd == nil || !pd.Enabled() {
		return nil, "", nil
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, "", err
	}
	b := &LocalHTTPBridge{listener: ln, dialer: pd}
	go b.serve()
	return b, "http://" + ln.Addr().String(), nil
}

func (b *LocalHTTPBridge) Close() error {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	b.closed = true
	b.mu.Unlock()
	return b.listener.Close()
}

func (b *LocalHTTPBridge) serve() {
	srv := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodConnect {
				targetConn, err := b.dialer.DialContext(r.Context(), "tcp", r.Host)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadGateway)
					return
				}
				hj, ok := w.(http.Hijacker)
				if !ok {
					_ = targetConn.Close()
					http.Error(w, "hijack unsupported", http.StatusInternalServerError)
					return
				}
				clientConn, _, err := hj.Hijack()
				if err != nil {
					_ = targetConn.Close()
					return
				}
				_, _ = clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
				go func() {
					_, _ = io.Copy(targetConn, clientConn)
					_ = targetConn.Close()
				}()
				go func() {
					_, _ = io.Copy(clientConn, targetConn)
					_ = clientConn.Close()
				}()
				return
			}
			outReq := r.Clone(r.Context())
			outReq.RequestURI = ""
			client := b.dialer.HTTPClient(25 * time.Second)
			resp, err := client.Do(outReq)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadGateway)
				return
			}
			defer resp.Body.Close()
			for k, vv := range resp.Header {
				for _, v := range vv {
					w.Header().Add(k, v)
				}
			}
			w.WriteHeader(resp.StatusCode)
			_, _ = io.Copy(w, resp.Body)
		}),
	}
	_ = srv.Serve(b.listener)
}
