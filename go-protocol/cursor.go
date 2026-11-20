package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

// Cursor provider 的授权流程（与 MiniMax Code 并列的第二种面板 provider）：
//
//	1) 面板 /api/plugin-signin {provider:"cursor"} → 得到 deep-control 授权页 URL
//	2) 浏览器打开该页面 → 点「Continue to sign in」→ 进入 Cursor 登录页
//	3) 填邮箱 → Cursor 发 6 位验证码到邮箱（Graph 号池取码）
//	4) 填验证码 → 登录完成 → 页面回调 /api/auth/loginDeepCallbackControl
//	5) 面板 pending 的 /auth/poll 拿到 accessToken → 面板落库
//	6) 轮询面板 /api/signin/<id> 直到 done，账号出现在 cursor provider 的 logins 里

const cursorProviderID = "cursor"

// StartPluginSignInProvider 向面板申请某个 provider 的授权会话。
func (p *PanelClient) StartPluginSignInProvider(ctx context.Context, provider string) (*PluginSignInResp, error) {
	payload, _ := json.Marshal(map[string]any{
		"provider": provider,
		"method":   0,
		"inputs":   map[string]any{},
		"proxy":    "",
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/api/plugin-signin", strings.NewReader(string(payload)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("POST /api/plugin-signin(%s) HTTP %d: %s", provider, resp.StatusCode, string(body))
	}
	var out PluginSignInResp
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("解析 plugin-signin 失败: %w", err)
	}
	if out.ID == "" || out.URL == "" {
		return nil, fmt.Errorf("plugin-signin(%s) 返回缺少 id/url: %s", provider, string(body))
	}
	return &out, nil
}

var cursorEmailPattern = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

// extractCursorLogins 从 /api/providers 里取出已绑定的 Cursor 账号（邮箱）。
func extractCursorLogins(providers map[string]any) []string {
	set := map[string]struct{}{}
	add := func(v any) {
		s, ok := v.(string)
		if !ok {
			return
		}
		s = strings.TrimSpace(s)
		if cursorEmailPattern.MatchString(s) {
			set[s] = struct{}{}
		}
	}
	provs, _ := providers["providers"].([]any)
	for _, pv := range provs {
		pm, ok := pv.(map[string]any)
		if !ok {
			continue
		}
		if id, _ := pm["id"].(string); id != cursorProviderID {
			continue
		}
		if acc, ok := pm["account"].(map[string]any); ok {
			logins, _ := acc["logins"].([]any)
			for _, lv := range logins {
				if lm, ok := lv.(map[string]any); ok {
					add(lm["user"])
				}
			}
			add(acc["user"])
		}
	}
	out := make([]string, 0, len(set))
	for u := range set {
		out = append(out, u)
	}
	return out
}

// ListCursorUsers 列出面板上已绑定的 Cursor 账号。
func (p *PanelClient) ListCursorUsers(ctx context.Context) ([]string, error) {
	providers, err := p.CheckProviders(ctx)
	if err != nil {
		return nil, err
	}
	return extractCursorLogins(providers), nil
}

// WaitForCursorBinding 等面板把 Cursor 账号落库，返回绑定成功的邮箱。
func (p *PanelClient) WaitForCursorBinding(ctx context.Context, signInID string, before []string, timeout time.Duration) (string, error) {
	known := map[string]struct{}{}
	for _, u := range before {
		known[u] = struct{}{}
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/api/signin/"+url.PathEscape(signInID), nil)
		if err != nil {
			return "", err
		}
		if resp, err := p.http.Do(req); err == nil {
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode == 200 {
				var out SignInPollResp
				if json.Unmarshal(body, &out) == nil {
					switch out.State {
					case "done":
						if u := strings.TrimSpace(out.User); u != "" {
							return u, nil
						}
					case "error", "expired", "failed":
						return "", fmt.Errorf("面板 sign-in 终态异常: %s (%s)", out.State, out.Error)
					}
				}
			}
		}
		if users, err := p.ListCursorUsers(ctx); err == nil {
			for _, u := range users {
				if _, ok := known[u]; ok {
					continue
				}
				return u, nil
			}
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return "", fmt.Errorf("轮询 /api/signin/%s 超时", signInID)
}

// CursorSignInState 读一次 sign-in 记录的状态（waiting/done/error）。
func (p *PanelClient) CursorSignInState(ctx context.Context, signInID string) (string, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/api/signin/"+url.PathEscape(signInID), nil)
	if err != nil {
		return "", "", err
	}
	resp, err := p.http.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", "", fmt.Errorf("GET /api/signin/%s -> %d", signInID, resp.StatusCode)
	}
	var out SignInPollResp
	if err := json.Unmarshal(body, &out); err != nil {
		return "", "", err
	}
	return out.State, out.User, nil
}

// ============================ 浏览器驱动 ============================

// cursorBrowser 用 CDP 驱动一个真实 Chrome 走完 Cursor 的 deep-control 登录。
type cursorBrowser struct {
	cmd    *exec.Cmd
	conn   *websocket.Conn
	cancel context.CancelFunc
	msgID  int
	// profileDir 是本次运行用的临时 Chrome 配置目录，结束时删掉。
	profileDir string
	// bws 是浏览器级 CDP 连接，用来开新标签页。
	bws    *websocket.Conn
	bmsgID int
}

// cursorPageState 是页面快照，用来决定下一步点哪里。
type cursorPageState struct {
	URL      string   `json:"url"`
	HasEmail bool     `json:"hasEmail"`
	HasCode  bool     `json:"hasCode"`
	Buttons  []string `json:"buttons"`
	Text     string   `json:"text"`
	Done     bool     `json:"done"`
}

const cursorPageHelperJS = `
(function () {
  function setNativeValue(el, value) {
    var proto = (el.tagName === 'TEXTAREA') ? HTMLTextAreaElement.prototype : HTMLInputElement.prototype;
    var desc = Object.getOwnPropertyDescriptor(proto, 'value');
    if (desc && desc.set) { desc.set.call(el, value); }
    else { el.value = value; }
    el.dispatchEvent(new Event('input', { bubbles: true }));
    el.dispatchEvent(new Event('change', { bubbles: true }));
  }
  function visible(el) {
    if (!el) { return false; }
    if (el.disabled) { return false; }
    var r = el.getBoundingClientRect();
    if (r.width <= 0 && r.height <= 0) { return false; }
    var st = window.getComputedStyle(el);
    return st.visibility !== 'hidden' && st.display !== 'none' && st.opacity !== '0';
  }
  function pick(kind) {
    var sels = kind === 'email'
      ? ['input[type="email"]','input[name="email"]','input[id*="email" i]','input[placeholder*="mail" i]','input[autocomplete="email"]']
      : ['input[autocomplete="one-time-code"]','input[name="code"]','input[id*="code" i]','input[placeholder*="code" i]','input[inputmode="numeric"]','input[maxlength="6"]'];
    for (var i = 0; i < sels.length; i++) {
      var el = document.querySelector(sels[i]);
      if (visible(el)) { return el; }
    }
    return null;
  }
  window.__mmSet = function (kind, value) {
    var el = pick(kind);
    if (!el) { return false; }
    el.focus();
    setNativeValue(el, value);
    return true;
  };
  window.__mmClick = function (words) {
    var all = Array.prototype.slice.call(document.querySelectorAll('button, a[role="button"], [role="button"], input[type="submit"]'));
    for (var w = 0; w < words.length; w++) {
      var word = words[w].toLowerCase();
      for (var i = 0; i < all.length; i++) {
        var el = all[i];
        if (!visible(el)) { continue; }
        var t = (el.innerText || el.value || el.textContent || '').trim().toLowerCase();
        if (t && t.indexOf(word) >= 0) { el.click(); return t; }
      }
    }
    return '';
  };
  window.__mmState = function () {
    var email = pick('email');
    var code = pick('code');
    var btns = Array.prototype.slice.call(document.querySelectorAll('button'))
      .map(function (b) { return (b.innerText || '').trim(); })
      .filter(function (t) { return t.length > 0; });
    var body = (document.body && document.body.innerText) || '';
    return {
      url: location.href,
      hasEmail: !!email,
      hasCode: !!code,
      buttons: btns.slice(0, 16),
      text: body.slice(0, 400),
      done: /click continue to sign in|sign in to cursor cli|you'?re signed in|已登录/i.test(body)
    };
  };
})();
`

// sendWait 发送一条 CDP 命令并等待它的响应。
func (b *cursorBrowser) sendWait(ctx context.Context, method string, params map[string]any) (map[string]any, error) {
	if b.conn == nil {
		return nil, fmt.Errorf("浏览器连接未建立")
	}
	b.msgID++
	if params == nil {
		params = map[string]any{}
	}
	raw, _ := json.Marshal(map[string]any{"id": b.msgID, "method": method, "params": params})
	_ = b.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if err := b.conn.WriteMessage(websocket.TextMessage, raw); err != nil {
		return nil, err
	}
	for {
		_ = b.conn.SetReadDeadline(time.Now().Add(20 * time.Second))
		_, data, err := b.conn.ReadMessage()
		if err != nil {
			return nil, err
		}
		var resp map[string]any
		if json.Unmarshal(data, &resp) != nil {
			continue
		}
		if meth, _ := resp["method"].(string); meth != "" {
			cursorCDPEvent(meth, data)
			continue
		}
		if id, _ := resp["id"].(float64); int(id) == b.msgID {
			return resp, nil
		}
	}
}

// send 发送一条 CDP 命令（不等待结果）。
func (b *cursorBrowser) send(method string, params map[string]any) {
	if b.conn == nil {
		return
	}
	b.msgID++
	if params == nil {
		params = map[string]any{}
	}
	raw, _ := json.Marshal(map[string]any{"id": b.msgID, "method": method, "params": params})
	_ = b.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	_ = b.conn.WriteMessage(websocket.TextMessage, raw)
}

func (b *cursorBrowser) evalJS(ctx context.Context, expr string, awaitPromise bool) (any, error) {
	if b.conn == nil {
		return nil, fmt.Errorf("浏览器连接未建立")
	}
	b.msgID++
	req := map[string]any{
		"id":     b.msgID,
		"method": "Runtime.evaluate",
		"params": map[string]any{
			"expression":    expr,
			"returnByValue": true,
			"awaitPromise":  awaitPromise,
		},
	}
	raw, _ := json.Marshal(req)
	_ = b.conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
	if err := b.conn.WriteMessage(websocket.TextMessage, raw); err != nil {
		return nil, err
	}
	for {
		_ = b.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		_, data, err := b.conn.ReadMessage()
		if err != nil {
			return nil, err
		}
		var resp map[string]any
		if json.Unmarshal(data, &resp) != nil {
			continue
		}
		if meth, _ := resp["method"].(string); meth != "" {
			cursorCDPEvent(meth, data)
			continue
		}
		if id, _ := resp["id"].(float64); int(id) != b.msgID {
			continue
		}
		res, _ := resp["result"].(map[string]any)
		if res == nil {
			return nil, fmt.Errorf("CDP 返回异常: %s", string(data))
		}
		if ex, ok := res["exceptionDetails"]; ok {
			return nil, fmt.Errorf("页面脚本异常: %v", ex)
		}
		rv, _ := res["result"].(map[string]any)
		if rv == nil {
			return nil, nil
		}
		if val, ok := rv["value"]; ok {
			return val, nil
		}
		return nil, nil
	}
}

func (b *cursorBrowser) state(ctx context.Context) (*cursorPageState, error) {
	v, err := b.evalJS(ctx, "window.__mmState ? window.__mmState() : null", false)
	if err != nil {
		return nil, err
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("页面状态读取失败")
	}
	raw, _ := json.Marshal(m)
	var st cursorPageState
	_ = json.Unmarshal(raw, &st)
	return &st, nil
}

func (b *cursorBrowser) setInput(ctx context.Context, kind, value string) (bool, error) {
	v, err := b.evalJS(ctx, fmt.Sprintf("window.__mmSet(%q, %q)", kind, value), false)
	if err != nil {
		return false, err
	}
	ok, _ := v.(bool)
	return ok, nil
}

func (b *cursorBrowser) click(ctx context.Context, words ...string) (string, error) {
	list, _ := json.Marshal(words)
	v, err := b.evalJS(ctx, "window.__mmClick("+string(list)+")", false)
	if err != nil {
		return "", err
	}
	s, _ := v.(string)
	return s, nil
}

func (b *cursorBrowser) close() {
	if b.bws != nil {
		_ = b.bws.Close()
	}
	if b.conn != nil {
		_ = b.conn.Close()
	}
	if b.cmd != nil && b.cmd.Process != nil {
		killProcessTree(b.cmd.Process.Pid)
		_, _ = b.cmd.Process.Wait()
	}
	if b.cancel != nil {
		b.cancel()
	}
	if len(b.profileDir) > 0 {
		_ = os.RemoveAll(b.profileDir)
	}
}

// cursorManualHuman 表示由用户手动完成人机验证（MINIMAX_CURSOR_MANUAL=1）。
// 程序不点击 Turnstile，只等待用户点完继续。
var cursorManualHuman = os.Getenv("MINIMAX_CURSOR_MANUAL") == "1"

// launchCursorBrowser 启动一个带代理桥的 Chrome 并打开指定页面。
func launchCursorBrowser(ctx context.Context, chromePath, proxyBridge, profileRoot, startURL string) (*cursorBrowser, error) {
	if strings.TrimSpace(chromePath) == "" {
		return nil, fmt.Errorf("Chrome 路径未配置")
	}
	port, err := freeLocalPort()
	if err != nil {
		return nil, err
	}
	profileDir := filepath.Join(profileRoot, fmt.Sprintf("cursor-%d-%d", time.Now().UnixNano(), port))
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		return nil, err
	}
	bctx, cancel := context.WithCancel(ctx)
	args := []string{
		"--remote-debugging-port=" + strconv.Itoa(port),
		"--user-data-dir=" + profileDir,
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-background-networking",
		"--disable-background-timer-throttling",
		"--disable-backgrounding-occluded-windows",
		"--disable-renderer-backgrounding",
		"--disable-component-update",
		"--disable-domain-reliability",
		"--no-pings",
			"--window-size=1440,900",
	}
	// 窗口摆在屏幕左上角，人机验证由用户自己点。
	args = append(args, "--window-position=40,40")
	if runtime.GOOS != "windows" {
		args = append(args, "--no-sandbox")
	}
	if strings.TrimSpace(proxyBridge) != "" {
		args = append(args, "--proxy-server="+proxyBridge)
	}
	args = append(args, startURL)

	cmd := exec.CommandContext(bctx, chromePath, args...)
	cmd.SysProcAttr = chromeSysProcAttr()
	if err := cmd.Start(); err != nil {
		cancel()
		_ = os.RemoveAll(profileDir)
		return nil, fmt.Errorf("启动 Chrome 失败: %w", err)
	}
	b := &cursorBrowser{cmd: cmd, cancel: cancel, profileDir: profileDir}

	directHTTP := &http.Client{Transport: &http.Transport{Proxy: nil}, Timeout: 5 * time.Second}
	wsURL := ""
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			b.close()
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
					if t.Type == "page" && t.WebSocketDebuggerURL != "" {
						wsURL = t.WebSocketDebuggerURL
						break
					}
				}
			}
		}
		if wsURL != "" {
			break
		}
		time.Sleep(400 * time.Millisecond)
	}
	if wsURL == "" {
		b.close()
		return nil, fmt.Errorf("CDP 浏览器未就绪")
	}
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		b.close()
		return nil, fmt.Errorf("连接 CDP 失败: %w", err)
	}
	b.conn = conn
	if vresp, verr := directHTTP.Get(fmt.Sprintf("http://127.0.0.1:%d/json/version", port)); verr == nil {
		vraw, _ := io.ReadAll(vresp.Body)
		_ = vresp.Body.Close()
		var ver struct {
			WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
		}
		if json.Unmarshal(vraw, &ver) == nil && ver.WebSocketDebuggerURL != "" {
			if bconn, _, derr := websocket.DefaultDialer.DialContext(ctx, ver.WebSocketDebuggerURL, nil); derr == nil {
				b.bws = bconn
			}
		}
	}
	if _, err := b.evalJS(ctx, cursorPageHelperJS, false); err != nil {
		b.close()
		return nil, fmt.Errorf("注入页面脚本失败: %w", err)
	}
	return b, nil
}

// CursorResult 是一次 Cursor 注册的结果。
type CursorResult struct {
	SignInID string
	User     string
	Email    string
}

// CursorFlow 负责一次 Cursor 注册 + 授权。
type CursorFlow struct {
	Panel      *PanelClient
	ChromePath string
	ProxyBidge string
	ProfileDir string
	// Password 新建 Cursor 账号时设置的密码，留空则用随机强密码。
	Password string
	// FetchLink 取回邮箱里的确认链接（Cursor 的“确认变更账号”邮件）。
	FetchLink func(ctx context.Context, notBefore time.Time) (string, error)
}
