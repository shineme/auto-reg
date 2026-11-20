package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type TicketResult struct {
	Ticket   string            `json:"ticket"`
	Cookies  map[string]string `json:"cookies"`
	UUID     string            `json:"uuid,omitempty"`
	DeviceID string            `json:"deviceId,omitempty"`
}

type TicketProvider interface {
	AcquireTicket(ctx context.Context, targetEmail string, userCode string) (*TicketResult, error)
}

type ExternalTicketProvider struct {
	Endpoint string
	HTTP     *http.Client
}

func (e *ExternalTicketProvider) AcquireTicket(ctx context.Context, targetEmail string, userCode string) (*TicketResult, error) {
	if strings.TrimSpace(e.Endpoint) == "" {
		return nil, errors.New("外部票据接口 URL 未配置")
	}
	payload, _ := json.Marshal(map[string]any{
		"action":    "send_code",
		"email":     targetEmail,
		"user_code": userCode,
		"origin":    minimaxOrigin,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.Endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	client := e.HTTP
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("外部票据接口请求失败: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("外部票据接口 HTTP %d: %s", resp.StatusCode, string(raw))
	}
	var out TicketResult
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("外部票据接口返回 JSON 无效: %w", err)
	}
	if strings.TrimSpace(out.Ticket) == "" {
		return nil, fmt.Errorf("外部票据接口未返回 ticket: %s", string(raw))
	}
	if out.Cookies == nil {
		out.Cookies = map[string]string{}
	}
	return &out, nil
}

type PureGoCDPTicketProvider struct {
	ChromePath  string
	ProxyBridge string
	SiteKey     string
	ProfileRoot string
}

func freeLocalPort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port, nil
}

func (c *PureGoCDPTicketProvider) AcquireTicket(ctx context.Context, _ string, userCode string) (*TicketResult, error) {
	if strings.TrimSpace(c.ChromePath) == "" {
		return nil, errors.New("Chrome 路径未配置")
	}
	if strings.TrimSpace(c.SiteKey) == "" {
		return nil, errors.New("MINIMAX_RECAPTCHA_SITE_KEY 未配置")
	}
	port, err := freeLocalPort()
	if err != nil {
		return nil, err
	}
	profileDir := filepath.Join(c.ProfileRoot, fmt.Sprintf("cdp-%d-%d", time.Now().UnixNano(), port))
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		return nil, err
	}
	defer func() {
		_ = os.RemoveAll(profileDir)
	}()

	prewarmURL := minimaxOrigin + "/unified-login"
	if userCode != "" {
		lr := "/oauth-authorize?user_code=" + url.QueryEscape(userCode)
		prewarmURL = minimaxOrigin + "/unified-login?login_redirect=" + url.QueryEscape(lr)
	}

	args := []string{
		"--remote-debugging-port=" + strconv.Itoa(port),
		"--user-data-dir=" + profileDir,
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-background-networking",
		"--disable-background-timer-throttling",
		"--disable-backgrounding-occluded-windows",
		"--disable-renderer-backgrounding",
		"--disable-features=Translate",
		"--disable-component-update",
		"--disable-domain-reliability",
		"--no-pings",
		"--window-position=-32000,-32000",
		"--window-size=3440,1440",
	}
	if runtime.GOOS != "windows" {
		// Linux 下 root 运行 Chrome 必须关闭沙箱
		args = append(args, "--no-sandbox")
	}
	if strings.TrimSpace(c.ProxyBridge) != "" {
		args = append(args, "--proxy-server="+c.ProxyBridge)
	}
	args = append(args, prewarmURL)

	cmd := exec.CommandContext(ctx, c.ChromePath, args...)
	cmd.SysProcAttr = chromeSysProcAttr()
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("启动 Chrome 失败: %w", err)
	}
	defer func() {
		if cmd.Process != nil {
			killProcessTree(cmd.Process.Pid)
			_, _ = cmd.Process.Wait()
		}
	}()

	directHTTP := &http.Client{
		Transport: &http.Transport{Proxy: nil},
		Timeout:   5 * time.Second,
	}
	var browserWS string
	var targetID string
	deadline := time.Now().Add(25 * time.Second)
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	resp, err := directHTTP.Get(fmt.Sprintf("http://127.0.0.1:%d/json/list", port))
	if err == nil {
		raw, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		var targets []struct {
			Type                 string `json:"type"`
			URL                  string `json:"url"`
			ID                   string `json:"id"`
			WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
		}
		if json.Unmarshal(raw, &targets) == nil {
			for _, t := range targets {
				if t.Type == "page" && strings.Contains(t.URL, "account.minimax.io") && t.WebSocketDebuggerURL != "" {
					browserWS = t.WebSocketDebuggerURL
					targetID = t.ID
					break
				}
			}
		}
		if browserWS != "" && targetID != "" {
			break
		}
	}
		time.Sleep(400 * time.Millisecond)
	}
	if browserWS == "" {
		return nil, errors.New("CDP 浏览器未就绪")
	}

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, browserWS, nil)
	if err != nil {
		return nil, fmt.Errorf("连接 CDP WebSocket 失败: %w", err)
	}
	defer conn.Close()

	cdp := &cdpSession{conn: conn}
	attachTarget, err := cdp.call(ctx, "Target.attachToTarget", map[string]any{
		"targetId": targetID,
		"flatten":  true,
	})
	if err != nil {
		return nil, fmt.Errorf("连接 CDP 页面会话失败: %w", err)
	}
	sessionID, _ := attachTarget["sessionId"].(string)
	if sessionID == "" {
		return nil, errors.New("CDP 未返回页面 sessionId")
	}
	for _, command := range []struct {
		method string
		params any
	}{
		{"Page.enable", nil},
		{"Runtime.enable", nil},
		{"Network.enable", nil},
		{"Network.setCacheDisabled", map[string]any{"cacheDisabled": true}},
	} {
		if _, err := cdp.call(ctx, command.method, command.params, sessionID); err != nil {
			return nil, fmt.Errorf("启用 %s 失败: %w", command.method, err)
		}
	}
	if _, err := cdp.call(ctx, "Page.navigate", map[string]any{
		"url": prewarmURL,
	}, sessionID); err != nil {
		return nil, fmt.Errorf("打开 reCAPTCHA 预热页失败: %w", err)
	}

	ready := false
	injected := false
	injectExpr := fmt.Sprintf(`(function () {
		if (window.grecaptcha && window.grecaptcha.enterprise) return true;
		if (document.querySelector('script[data-mmx-captcha="1"]')) return true;
		var s = document.createElement("script");
		s.src = "https://www.google.com/recaptcha/enterprise.js?render=%s";
		s.async = true;
		s.dataset.mmxCaptcha = "1";
		(document.head || document.documentElement).appendChild(s);
		return true;
	})()`, url.QueryEscape(c.SiteKey))
	var lastState any
	for i := 0; i < 240; i++ {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		val, err := cdp.call(ctx, "Runtime.evaluate", map[string]any{
			"expression": `({href: location.href, ready: document.readyState, grecaptcha: Boolean(window.grecaptcha && window.grecaptcha.enterprise)})`,
			"returnByValue": true,
			"awaitPromise":  false,
			"userGesture":   true,
		}, sessionID)
		if err != nil {
			// If the socket times out while waiting, reconnect the browser session.
			_ = conn.Close()
			reconn, _, derr := websocket.DefaultDialer.DialContext(ctx, browserWS, nil)
			if derr != nil {
				return nil, fmt.Errorf("CDP 连接断开且重连失败: %w", derr)
			}
			conn = reconn
			cdp = &cdpSession{conn: reconn}
			lastState = map[string]any{"error": err.Error()}
			time.Sleep(500 * time.Millisecond)
			continue
		}
		lastState = extractCDPValue(val)
		if m, ok := lastState.(map[string]any); ok {
			gr, _ := m["grecaptcha"].(bool)
			rs, _ := m["ready"].(string)
			if !gr && !injected && rs != "" && rs != "loading" {
				if _, ierr := cdp.call(ctx, "Runtime.evaluate", map[string]any{
					"expression":   injectExpr,
					"returnByValue": true,
				}, sessionID); ierr == nil {
					injected = true
				}
			}
			if gr && strings.Contains(fmt.Sprint(m["href"]), "account.minimax.io") {
				ready = true
				break
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	if !ready {
		return nil, fmt.Errorf("reCAPTCHA Enterprise did not become ready (state=%v)", lastState)
	}
	// Give Akamai sensor a short window to mint _abck/bm_sz before we execute captcha.
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(2 * time.Second):
	}

	jsExec := fmt.Sprintf(`(async () => {
		await new Promise((resolve) => window.grecaptcha.enterprise.ready(resolve));
		const token = await window.grecaptcha.enterprise.execute(%q, { action: "send_code" });
		return {
			ticket: token || "",
			uuid: localStorage.getItem("mmx_account_UNIQUE_USER_ID") || "",
			deviceId: localStorage.getItem("mmx_account_USER_HARD_WARE_INFO") || ""
		};
	})()`, c.SiteKey)
	execResp, execErr := cdp.call(ctx, "Runtime.evaluate", map[string]any{
		"expression":    jsExec,
		"returnByValue": true,
		"awaitPromise":  true,
		"userGesture":   true,
	}, sessionID)
	if execErr != nil {
		_ = conn.Close()
		reconn, _, derr := websocket.DefaultDialer.DialContext(ctx, browserWS, nil)
		if derr != nil {
			return nil, fmt.Errorf("CDP 连接断开且重连失败: %w", derr)
		}
		conn = reconn
		cdp = &cdpSession{conn: reconn}
		return nil, fmt.Errorf("执行 grecaptcha.enterprise.execute 失败: %w", execErr)
	}
	execVal := extractCDPValue(execResp)
	execMap, _ := execVal.(map[string]any)
	ticket, _ := execMap["ticket"].(string)
	if strings.TrimSpace(ticket) == "" {
		return nil, errors.New("reCAPTCHA Enterprise returned no ticket")
	}

	cookieResp, err := cdp.call(ctx, "Network.getCookies", map[string]any{
		"urls": []string{minimaxOrigin + "/"},
	}, sessionID)
	if err != nil {
		return nil, fmt.Errorf("读取 CDP Cookies 失败: %w", err)
	}
	cookies := map[string]string{}
	if rawList, ok := cookieResp["cookies"].([]any); ok {
		for _, item := range rawList {
			if cm, ok := item.(map[string]any); ok {
				name, _ := cm["name"].(string)
				val, _ := cm["value"].(string)
				if name != "" {
					cookies[name] = val
				}
			}
		}
	}
	if cookies["_abck"] == "" {
		if allResp, aerr := cdp.call(ctx, "Network.getAllCookies", nil, sessionID); aerr == nil {
			if rawList, ok := allResp["cookies"].([]any); ok {
				for _, item := range rawList {
					if cm, ok := item.(map[string]any); ok {
						name, _ := cm["name"].(string)
						val, _ := cm["value"].(string)
						domain, _ := cm["domain"].(string)
						if name != "" && strings.Contains(domain, "minimax.io") {
							cookies[name] = val
						}
					}
				}
			}
		}
	}

	uuid, _ := execMap["uuid"].(string)
	deviceID, _ := execMap["deviceId"].(string)
	return &TicketResult{
		Ticket:   ticket,
		Cookies:  cookies,
		UUID:     uuid,
		DeviceID: deviceID,
	}, nil
}

type cdpSession struct {
	conn *websocket.Conn
	mu   sync.Mutex
	seq  int
}

type cdpSessionContext struct {
	session *cdpSession
	id      string
}

func (s *cdpSession) call(ctx context.Context, method string, params any, sessionID ...string) (map[string]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seq++
	id := s.seq
	req := map[string]any{
		"id":     id,
		"method": method,
	}
	if params != nil {
		req["params"] = params
	}
	if len(sessionID) > 0 && strings.TrimSpace(sessionID[0]) != "" {
		req["sessionId"] = sessionID[0]
	}
	if dl, ok := ctx.Deadline(); ok {
		_ = s.conn.SetWriteDeadline(dl)
		_ = s.conn.SetReadDeadline(dl)
	} else {
		_ = s.conn.SetWriteDeadline(time.Now().Add(45 * time.Second))
		_ = s.conn.SetReadDeadline(time.Now().Add(45 * time.Second))
	}
	if err := s.conn.WriteJSON(req); err != nil {
		return nil, err
	}
	for {
		_, data, err := s.conn.ReadMessage()
		if err != nil {
			return nil, err
		}
		var msg struct {
			ID     int            `json:"id"`
			Result map[string]any `json:"result"`
			Error  *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(data, &msg) != nil {
			continue
		}
		if msg.ID != id {
			continue
		}
		if msg.Error != nil {
			return nil, errors.New(msg.Error.Message)
		}
		return msg.Result, nil
	}
}

func (s *cdpSession) eval(ctx context.Context, expr string, awaitPromise bool) (any, error) {
	res, err := s.call(ctx, "Runtime.evaluate", map[string]any{
		"expression":    expr,
		"returnByValue": true,
		"awaitPromise":  awaitPromise,
		"userGesture":   true,
	})
	if err != nil {
		return nil, err
	}
	if ex, ok := res["exceptionDetails"].(map[string]any); ok {
		return nil, fmt.Errorf("JS 异常: %v", ex)
	}
	r, _ := res["result"].(map[string]any)
	return r["value"], nil
}

func extractCDPValue(resp map[string]any) any {
	if resp == nil {
		return nil
	}
	if result, ok := resp["result"].(map[string]any); ok {
		return result["value"]
	}
	return nil
}
