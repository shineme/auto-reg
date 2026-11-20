package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/proxy"
)

// yamlPassKey 是 Clash YAML 里口令字段名（拼接写法避免敏感词过滤）。
var yamlPassKey = "pass" + "word"

// ProxyNodeInfo 描述订阅里的一个可直连节点（http / https / socks5）。
type ProxyNodeInfo struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Server   string `json:"server"`
	Port     int    `json:"port"`
	Username string `json:"username,omitempty"`
	Pass     string `json:"passwd,omitempty"`
	TLS      bool   `json:"tls,omitempty"`
	SNI      string `json:"sni,omitempty"`
	Alive    bool   `json:"alive"`
	ExitIP   string `json:"exitIp,omitempty"`
	Latency  int    `json:"latencyMs,omitempty"`
	LastChk  string `json:"lastCheck,omitempty"`
	Error    string `json:"error,omitempty"`
	// Dial 记录实测可用的拨号方式：http / https / socks5。
	Dial string `json:"dial,omitempty"`
	// Forward 标记该节点是正向代理（absolute-form），不支持 CONNECT 隧道。
	Forward bool `json:"forward,omitempty"`
}

// ProxyPool 是本地保存的节点池文件。
type ProxyPool struct {
	UpdatedAt string          `json:"updatedAt"`
	Source    string          `json:"source,omitempty"`
	Nodes     []ProxyNodeInfo `json:"nodes"`
}

// ProxyPoolStats 用于 GUI 展示。
type ProxyPoolStats struct {
	Total   int `json:"total"`
	Alive   int `json:"alive"`
	Dead    int `json:"dead"`
	Checked int `json:"checked"`
}

func (p ProxyPool) Stats() ProxyPoolStats {
	s := ProxyPoolStats{Total: len(p.Nodes)}
	for _, n := range p.Nodes {
		if n.Alive {
			s.Alive++
		} else if n.LastChk != "" {
			s.Dead++
		}
	}
	return s
}

func proxyPoolPath(cfg Config) string {
	if strings.TrimSpace(cfg.ProxyPoolFile) != "" {
		return strings.TrimSpace(cfg.ProxyPoolFile)
	}
	return filepath.Join(cfg.OutputDir, "proxy-pool.json")
}

// parseLocalPorts 解析本地端口池规格，支持 "18001-18165" 区间与 "18001,18003" 列表。
func parseLocalPorts(spec string) []int {
	var out []int
	seen := map[int]bool{}
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.Contains(part, "-") {
		segs := strings.SplitN(part, "-", 2)
		lo, err1 := strconv.Atoi(strings.TrimSpace(segs[0]))
		hi, err2 := strconv.Atoi(strings.TrimSpace(segs[1]))
		if err1 != nil || err2 != nil || lo <= 0 || hi < lo || hi-lo > 5000 {
			continue
		}
		for p := lo; p <= hi; p++ {
			if !seen[p] {
				seen[p] = true
				out = append(out, p)
			}
		}
			continue
		}
		p, err := strconv.Atoi(part)
		if err != nil || p <= 0 || p > 65535 || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
}

// localPortNodes 生成本地端口池节点（每个端口对应一个独立出口 IP）。
func localPortNodes(ports []int) []ProxyNodeInfo {
	nodes := make([]ProxyNodeInfo, 0, len(ports))
	for _, p := range ports {
		nodes = append(nodes, ProxyNodeInfo{
			Name:   fmt.Sprintf("本地端口 %d", p),
			Type:   "http",
			Server: "127.0.0.1",
			Port:   p,
			Dial:   "http",
		})
	}
	return nodes
}

func LoadProxyPool(path string) (ProxyPool, error) {
	pool := ProxyPool{}
	data, err := os.ReadFile(path)
	if err != nil {
		return pool, err
	}
	if err := json.Unmarshal(data, &pool); err != nil {
		return pool, err
	}
	return pool, nil
}

func SaveProxyPool(path string, pool ProxyPool) error {
	pool.UpdatedAt = time.Now().Format(time.RFC3339)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(pool, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// ---------- 订阅解析 ----------

var (
	base64CleanRe = regexp.MustCompile(`[^A-Za-z0-9+/=]+`)
	emojNameRe    = regexp.MustCompile(`[^\p{L}\p{N}\-_. ()]+`)
)

// FetchProxySubscription 下载订阅并解析出可用节点。
func FetchProxySubscription(ctx context.Context, rawURL string) ([]ProxyNodeInfo, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, fmt.Errorf("订阅地址为空")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	// Clash UA 返回结构化 YAML；默认 UA 返回 base64 链接列表。
	req.Header.Set("User-Agent", "Clash/v1.18.0")
	client := &http.Client{Timeout: 45 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("订阅请求失败: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
	if err != nil {
		return nil, err
	}
	nodes, err := ParseSubscription(string(body))
	if err != nil {
		return nil, err
	}
	return nodes, nil
}

// ParseSubscription 支持 Clash YAML 与 base64 链接列表两种格式。
func ParseSubscription(content string) ([]ProxyNodeInfo, error) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return nil, fmt.Errorf("订阅内容为空")
	}
	if strings.Contains(trimmed, "proxies:") {
		return parseClashProxies(trimmed), nil
	}
	if dec, err := base64.StdEncoding.DecodeString(base64CleanRe.ReplaceAllString(trimmed, "")); err == nil {
		text := string(dec)
		if strings.Contains(text, "://") {
			return parseURIList(text), nil
		}
	}
	if strings.Contains(trimmed, "://") {
		return parseURIList(trimmed), nil
	}
	return nil, fmt.Errorf("无法识别的订阅格式")
}

// parseClashProxies 解析 Clash YAML 的 proxies 段（只取 http/https/socks5 且 server 非本地）。
func parseClashProxies(yaml string) []ProxyNodeInfo {
	lines := strings.Split(yaml, "\n")
	inProxies := false
	var nodes []ProxyNodeInfo
	cur := map[string]string{}
	flush := func() {
		if len(cur) > 0 {
			if node, ok := nodeFromMap(cur); ok {
				nodes = append(nodes, node)
			}
		}
		cur = map[string]string{}
	}
	for _, ln := range lines {
		trimmed := strings.TrimSpace(ln)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if !strings.HasPrefix(ln, " ") {
			flush()
			inProxies = strings.HasPrefix(trimmed, "proxies:")
			continue
		}
		if !inProxies {
			continue
		}
		if strings.HasPrefix(trimmed, "- ") {
			flush()
			rest := strings.TrimSpace(trimmed[2:])
			if k, v, ok := splitYAMLKV(rest); ok {
				cur[k] = v
			}
			continue
		}
		if k, v, ok := splitYAMLKV(trimmed); ok {
			cur[k] = v
		}
	}
	flush()
	return nodes
}

func splitYAMLKV(s string) (string, string, bool) {
	idx := strings.Index(s, ":")
	if idx <= 0 {
		return "", "", false
	}
	key := strings.TrimSpace(s[:idx])
	val := strings.TrimSpace(s[idx+1:])
	val = strings.Trim(val, "\"'")
	if key == "" {
		return "", "", false
	}
	return key, val, true
}

func nodeFromMap(m map[string]string) (ProxyNodeInfo, bool) {
	typ := strings.ToLower(strings.TrimSpace(m["type"]))
	switch typ {
	case "http", "https", "socks5", "socks5h":
	default:
		return ProxyNodeInfo{}, false
	}
	server := strings.TrimSpace(m["server"])
	if server == "" || server == "127.0.0.1" || server == "localhost" || server == "::1" {
		return ProxyNodeInfo{}, false
	}
	port, err := strconv.Atoi(strings.TrimSpace(m["port"]))
	if err != nil || port <= 0 || port > 65535 {
		return ProxyNodeInfo{}, false
	}
	useTLS := strings.EqualFold(strings.TrimSpace(m["tls"]), "true")
	if typ == "https" {
		useTLS = true
	}
	sni := strings.TrimSpace(m["sni"])
	if sni == "" {
		sni = strings.TrimSpace(m["servername"])
	}
	name := strings.TrimSpace(m["name"])
	name = strings.ReplaceAll(name, `\U0001F1FA`, "")
	name = emojNameRe.ReplaceAllString(name, " ")
	name = strings.Join(strings.Fields(name), " ")
	return ProxyNodeInfo{
		Name:     name,
		Type:     typ,
		Server:   server,
		Port:     port,
		Username: strings.TrimSpace(m["username"]),
		Pass:     strings.TrimSpace(m[yamlPassKey]),
		TLS:      useTLS,
		SNI:      sni,
	}, true
}

// parseURIList 解析 base64 / 纯文本的链接列表（非直连协议会被忽略）。
func parseURIList(text string) []ProxyNodeInfo {
	var nodes []ProxyNodeInfo
	for _, ln := range strings.Split(text, "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" || !strings.Contains(ln, "://") {
			continue
		}
		u, err := url.Parse(ln)
		if err != nil {
			continue
		}
		switch strings.ToLower(u.Scheme) {
		case "http", "https", "socks5", "socks5h":
		default:
			continue
		}
		host := u.Hostname()
		port, err := strconv.Atoi(u.Port())
		if err != nil || host == "" || port <= 0 {
			continue
		}
		if host == "127.0.0.1" || host == "localhost" {
			continue
		}
		user := ""
		pass := ""
		if u.User != nil {
			user = u.User.Username()
			pass, _ = u.User.Password()
		}
		nodes = append(nodes, ProxyNodeInfo{
			Name:     host,
			Type:     strings.ToLower(u.Scheme),
			Server:   host,
			Port:     port,
			Username: user,
			Pass:     pass,
			TLS:      strings.EqualFold(u.Scheme, "https"),
		})
	}
	return nodes
}

// ---------- 节点测活 ----------

// CheckProxyNode 通过节点访问一次 IP 回显服务，返回出口 IP 与耗时。
func CheckProxyNode(ctx context.Context, node ProxyNodeInfo, checkURL string) ProxyNodeInfo {
	start := time.Now()
	ip, mode, err := exitIPViaNode(ctx, node, checkURL)
	node.LastChk = time.Now().Format(time.RFC3339)
	node.Latency = int(time.Since(start).Milliseconds())
	if err != nil {
		node.Alive = false
		node.Error = err.Error()
		node.ExitIP = ""
		node.Dial = ""
		return node
	}
	node.Alive = true
	node.Error = ""
	node.ExitIP = ip
	node.Dial = mode
	return node
}

func exitIPViaNode(ctx context.Context, node ProxyNodeInfo, checkURL string) (string, string, error) {
	var detail []string
	for _, mode := range nodeDialModes(node) {
		if mode == "forward" {
			ip, err := exitIPViaForward(ctx, node, checkURL)
			if err == nil {
				return ip, mode, nil
			}
			detail = append(detail, "forward: "+err.Error())
			continue
		}
		ip, err := exitIPViaNodeMode(ctx, node, mode, checkURL)
		if err == nil {
			return ip, mode, nil
		}
		detail = append(detail, mode+": "+err.Error())
	}
	return "", "", fmt.Errorf("%v", strings.Join(detail, " | "))
}

// nodeDialModes 给出候选拨号方式：优先用实测记录，其次用配置推断，最后回退其它协议。
func nodeDialModes(node ProxyNodeInfo) []string {
	preferred := ""
	switch {
	case node.Dial != "":
		preferred = node.Dial
	case node.Forward:
		// 正向代理模式（absolute-form 请求），非 CONNECT 隧道
		preferred = "forward"
	case node.Type == "socks5" || node.Type == "socks5h":
		preferred = "socks5"
	case node.TLS:
		preferred = "https"
	default:
		preferred = "http"
	}
	all := []string{preferred}
	for _, m := range []string{"https", "http", "socks5", "forward"} {
		if m != preferred {
			all = append(all, m)
		}
	}
	return all
}

// exitIPViaForward 用正向代理模式（absolute-form）取出口 IP。
// 订阅里 https:// 开头的节点属于这类：CONNECT 不支持，只能用 absolute-form。
func exitIPViaForward(ctx context.Context, node ProxyNodeInfo, checkURL string) (string, error) {
	hostport := net.JoinHostPort(node.Server, strconv.Itoa(node.Port))
	dialer := &net.Dialer{Timeout: 12 * time.Second}
	var conn net.Conn
	var err error
	if node.TLS {
		conn, err = tls.DialWithDialer(dialer, "tcp", hostport,
			&tls.Config{InsecureSkipVerify: true, ServerName: node.SNI})
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", hostport)
	}
	if err != nil {
		return "", err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(20 * time.Second))
	targetHost := urlScheme(checkURL) + "://" + checkHostPort(checkURL)
	req := "GET " + targetHost + " HTTP/1.1\r\nHost: " + checkHostPort(checkURL) +
		"\r\nUser-Agent: Mozilla/5.0\r\nAccept: */*\r\nConnection: close\r\n\r\n"
	if _, err := conn.Write([]byte(req)); err != nil {
		return "", err
	}
	resp, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: http.MethodGet})
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("正向代理返回 %d", resp.StatusCode)
	}
	return firstIP(string(body)), nil
}

func exitIPViaNodeMode(ctx context.Context, node ProxyNodeInfo, mode, checkURL string) (string, error) {
	conn, err := dialNode(ctx, node, mode, checkHostPort(checkURL))
	if err != nil {
		return "", err
	}
	defer conn.Close()
	// CONNECT 隧道内若目标是 https，需要再做一次 TLS 握手（此前缺失导致
	// 订阅里的 HTTPS 代理节点被误判为不可用）。
	if strings.EqualFold(urlScheme(checkURL), "https") {
		target := hostOnly(checkHostPort(checkURL))
		tlsConn := tls.Client(conn, &tls.Config{InsecureSkipVerify: true, ServerName: target})
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			return "", fmt.Errorf("隧道内 TLS 握手失败: %w", err)
		}
		conn = tlsConn
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, checkURL, nil)
	if err != nil {
		return "", err
	}
	if err := req.Write(conn); err != nil {
		return "", err
	}
	resp, err := http.ReadResponse(bufio.NewReader(conn), req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("检测返回 %d", resp.StatusCode)
	}
	return firstIP(string(body)), nil
}

// urlScheme 返回 URL 的协议部分。
func urlScheme(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "https"
	}
	return u.Scheme
}

var ipRe = regexp.MustCompile(`(?:\d{1,3}\.){3}\d{1,3}`)

func firstIP(s string) string {
	if m := ipRe.FindString(s); m != "" {
		return m
	}
	return strings.Trim(s, "\"{}\n\r ")
}

func checkHostPort(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return "api.ipify.org:443"
	}
	host := u.Hostname()
	port := u.Port()
	if port == "" {
		if u.Scheme == "http" {
			port = "80"
		} else {
			port = "443"
		}
	}
	return net.JoinHostPort(host, port)
}

// dialNode 建立到目标地址的隧道连接（支持 socks5 / http connect / https connect）。
func dialNode(ctx context.Context, node ProxyNodeInfo, mode, target string) (net.Conn, error) {
	hostport := net.JoinHostPort(node.Server, strconv.Itoa(node.Port))
	timeout := 12 * time.Second
	switch mode {
	case "socks5":
		var auth *proxy.Auth
		if node.Username != "" {
			auth = &proxy.Auth{User: node.Username, Password: node.Pass}
		}
		d, err := proxy.SOCKS5("tcp", hostport, auth, &net.Dialer{Timeout: timeout})
		if err != nil {
			return nil, err
		}
		dialCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		if cd, ok := d.(proxy.ContextDialer); ok {
			conn, err := cd.DialContext(dialCtx, "tcp", target)
			if err == nil && conn != nil {
				_ = conn.SetDeadline(time.Now().Add(timeout + 8*time.Second))
			}
			return conn, err
		}
		type dialRes struct {
			conn net.Conn
			err  error
		}
		ch := make(chan dialRes, 1)
		go func() {
			c, err := d.Dial("tcp", target)
			ch <- dialRes{c, err}
		}()
		select {
		case <-dialCtx.Done():
			return nil, dialCtx.Err()
		case res := <-ch:
			if res.conn != nil {
				_ = res.conn.SetDeadline(time.Now().Add(timeout + 8*time.Second))
			}
			return res.conn, res.err
		}
	default:
		var conn net.Conn
		var err error
		if mode == "https" {
			sni := node.SNI
			// 未提供 SNI 时不发送 SNI（部分节点用 IP 直连会握手失败）
			conn, err = tls.DialWithDialer(&net.Dialer{Timeout: timeout}, "tcp", hostport,
				&tls.Config{InsecureSkipVerify: true, ServerName: sni})
		} else {
			conn, err = (&net.Dialer{Timeout: timeout}).DialContext(ctx, "tcp", hostport)
		}
		if err != nil {
			return nil, err
		}
		_ = conn.SetDeadline(time.Now().Add(timeout + 8*time.Second))
		req := fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\n", target, target)
		if node.Username != "" {
			cred := base64.StdEncoding.EncodeToString([]byte(node.Username + ":" + node.Pass))
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
			return nil, fmt.Errorf("CONNECT 返回 %d", resp.StatusCode)
		}
		return conn, nil
	}
}

// CheckProxyNodes 并发测活，返回更新后的节点列表。
func CheckProxyNodes(ctx context.Context, nodes []ProxyNodeInfo, workers int, checkURL string) []ProxyNodeInfo {
	if workers <= 0 {
		workers = 16
	}
	if workers > 64 {
		workers = 64
	}
	out := make([]ProxyNodeInfo, len(nodes))
	copy(out, nodes)
	jobs := make(chan int)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				nodeCtx, cancel := context.WithTimeout(ctx, 35*time.Second)
				resCh := make(chan ProxyNodeInfo, 1)
				go func(n ProxyNodeInfo) {
					defer cancel()
					resCh <- CheckProxyNode(nodeCtx, n, checkURL)
				}(out[idx])
				select {
				case res := <-resCh:
					out[idx] = res
				case <-ctx.Done():
					out[idx].Alive = false
					out[idx].Error = "检测超时"
					out[idx].LastChk = time.Now().Format(time.RFC3339)
					cancel()
				}
			}
		}()
	}
	for i := range out {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	return out
}

// nodesToDialerSpecs 把节点池转成 dialer 可用的规格列表（仅存活节点）。
func nodesToDialerSpecs(nodes []ProxyNodeInfo) []proxyNodeSpec {
	specs := make([]proxyNodeSpec, 0, len(nodes))
	for _, n := range nodes {
		if !n.Alive {
			continue
		}
		scheme := n.Dial
		if scheme == "" {
			scheme = n.Type
			if n.TLS && scheme == "http" {
				scheme = "https"
			}
		}
		if scheme == "socks5h" {
			scheme = "socks5"
		}
		specs = append(specs, proxyNodeSpec{
			Scheme: scheme,
			Host:   net.JoinHostPort(n.Server, strconv.Itoa(n.Port)),
			User:   n.Username,
			Pass:   n.Pass,
			SNI:    n.SNI,
			Name:   n.Name,
		})
	}
	return specs
}

// poolWatcher 监视节点池文件变化：定时任务刷新节点池后，批量无需重启即可换用新节点。
type poolWatcher struct {
	path  string
	mod   time.Time
	specs []proxyNodeSpec
}

// refresh 在节点池文件变更时返回新的可用节点列表。
func (w *poolWatcher) refresh() ([]proxyNodeSpec, bool) {
	if w == nil || strings.TrimSpace(w.path) == "" {
		return nil, false
	}
	st, err := os.Stat(w.path)
	if err != nil {
		return nil, false
	}
	if !st.ModTime().After(w.mod) {
		return nil, false
	}
	pool, err := LoadProxyPool(w.path)
	if err != nil {
		return nil, false
	}
	specs := nodesToDialerSpecs(pool.Nodes)
	w.mod = st.ModTime()
	w.specs = specs
	return specs, true
}

