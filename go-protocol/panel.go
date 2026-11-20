package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

type PanelClient struct {
	baseURL string
	key     string
	http    *http.Client
	// ceremonyMu 每个面板独立的"授权+落库"仪式锁：多路时同一面板内串行，
	// 不同面板之间并行，避免多线程抢占同一个面板把 user_code/轮询搞崩。
	ceremonyMu sync.Mutex
	// claimedMu/claimedUsers 记录本面板已被认领的账号，避免同面板多线程抢到同一用户
	claimedMu    sync.Mutex
	claimedUsers map[string]struct{}
}

// CeremonyLock 进入该面板的绑定仪式（授权到落库完成）。
func (p *PanelClient) CeremonyLock() { p.ceremonyMu.Lock() }

// CeremonyUnlock 退出绑定仪式。
func (p *PanelClient) CeremonyUnlock() { p.ceremonyMu.Unlock() }

type PluginSignInResp struct {
	ID    string `json:"id"`
	URL   string `json:"url"`
	Code  string `json:"code"`
	State string `json:"state"`
	Error string `json:"error,omitempty"`
}

type SignInPollResp struct {
	ID    string `json:"id"`
	State string `json:"state"`
	User  string `json:"user"`
	Error string `json:"error,omitempty"`
}

func NewPanelClient(baseURL, key string, dialer *ProxyDialer, directHosts []string) (*PanelClient, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	u, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil {
		return nil, err
	}
	useProxy := dialer != nil && dialer.Enabled()
	if useProxy {
		h := strings.ToLower(u.Hostname())
		for _, dh := range directHosts {
			if strings.EqualFold(strings.TrimSpace(dh), h) {
				useProxy = false
				break
			}
		}
	}
	var client *http.Client
	if useProxy {
		client = dialer.HTTPClient(25 * time.Second)
	} else {
		client = &http.Client{Timeout: 25 * time.Second}
	}
	client.Jar = jar
	// Follow redirects manually during ?k= login so we inspect 303 Set-Cookie cleanly
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return &PanelClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		key:     key,
		http:    client,
	}, nil
}

func (p *PanelClient) Login(ctx context.Context) error {
	curURL := p.baseURL + "/?k=" + url.QueryEscape(p.key)
	for hop := 0; hop < 6; hop++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, curURL, nil)
		if err != nil {
			return err
		}
		resp, err := p.http.Do(req)
		if err != nil {
			return fmt.Errorf("面板登录请求失败: %w", err)
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode >= 300 && resp.StatusCode < 400 {
			loc := resp.Header.Get("Location")
			if loc == "" {
				break
			}
			next, err := url.Parse(loc)
			if err != nil {
				return err
			}
			base, _ := url.Parse(curURL)
			curURL = base.ResolveReference(next).String()
			continue
		}
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode >= 400 {
			return fmt.Errorf("面板登录失败 HTTP %d", resp.StatusCode)
		}
		return nil
	}
	return nil
}

func (p *PanelClient) CheckProviders(ctx context.Context) (map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/api/providers", nil)
	if err != nil {
		return nil, err
	}
	resp, err := p.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("GET /api/providers HTTP %d: %s", resp.StatusCode, string(body))
	}
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("解析 /api/providers 失败: %w", err)
	}
	return out, nil
}

func (p *PanelClient) StartPluginSignIn(ctx context.Context) (*PluginSignInResp, error) {
	payload, _ := json.Marshal(map[string]any{
		"provider": "minimax-code-global",
		"method":   0,
		"inputs":   map[string]any{},
		"proxy":    "",
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/api/plugin-signin", bytes.NewReader(payload))
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
		return nil, fmt.Errorf("POST /api/plugin-signin HTTP %d: %s", resp.StatusCode, string(body))
	}
	var out PluginSignInResp
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	if out.Code == "" || out.URL == "" {
		return nil, fmt.Errorf("plugin-signin 返回缺少 code/url: %s", string(body))
	}
	return &out, nil
}

func (p *PanelClient) PollSignIn(ctx context.Context, signInID string, timeout time.Duration) (*SignInPollResp, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/api/signin/"+url.PathEscape(signInID), nil)
		if err != nil {
			return nil, err
		}
		resp, err := p.http.Do(req)
		if err == nil {
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode == 200 {
				var out SignInPollResp
				if json.Unmarshal(body, &out) == nil {
					if out.State == "done" {
						return &out, nil
					}
					if out.State == "error" || out.State == "expired" || out.State == "failed" {
						return &out, fmt.Errorf("面板 sign-in 终态异常: %s (%s)", out.State, out.Error)
					}
				}
			}
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return nil, fmt.Errorf("轮询 /api/signin/%s 超时", signInID)
}

// miniMaxUserPattern matches real magpie provider usernames (MiniMax + digits).
// Other "MiniMax*" strings in the payload (provider display names, model groups
// like MiniMax-M3, and the testPicks cache) are NOT accounts and must be ignored.
var miniMaxUserPattern = regexp.MustCompile(`^MiniMax\d+$`)

// extractMiniMaxLogins pulls the real bound accounts out of /api/providers.
// The authoritative location is providers[].account.logins[].user; accountUsers
// holds the currently selected account. Walking the whole JSON (as the old
// collector did) also picked up testPicks (1000+ stale usernames) and model
// group names, which made the "new account appeared" detection unreliable.
func extractMiniMaxLogins(providers map[string]any) []string {
	set := map[string]struct{}{}
	add := func(v any) {
		s, ok := v.(string)
		if ok && miniMaxUserPattern.MatchString(strings.TrimSpace(s)) {
			set[strings.TrimSpace(s)] = struct{}{}
		}
	}
	provs, _ := providers["providers"].([]any)
	for _, pv := range provs {
		pm, ok := pv.(map[string]any)
		if !ok {
			continue
		}
		if id, _ := pm["id"].(string); id != "minimax-code-global" {
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
		if au, ok := pm["accountUsers"].(map[string]any); ok {
			for _, v := range au {
				switch vv := v.(type) {
				case string:
					add(vv)
				case []any:
					for _, item := range vv {
						if im, ok := item.(map[string]any); ok {
							add(im["name"])
						}
					}
				case map[string]any:
					add(vv["name"])
				}
			}
		}
	}
	out := make([]string, 0, len(set))
	for user := range set {
		out = append(out, user)
	}
	sort.Strings(out)
	return out
}

// collectMiniMaxUsers kept for compatibility: collects real accounts using the strict rule
func collectMiniMaxUsers(value any, users map[string]struct{}) {
	if providers, ok := value.(map[string]any); ok {
		for _, user := range extractMiniMaxLogins(providers) {
			users[user] = struct{}{}
		}
	}
}

func (p *PanelClient) ListMiniMaxUsers(ctx context.Context) ([]string, error) {
	providers, err := p.CheckProviders(ctx)
	if err != nil {
		return nil, err
	}
	return extractMiniMaxLogins(providers), nil
}

// claimMiniMaxUser returns true only if this user has not been bound by another
// account on THIS panel. The map is per panel instance on purpose: two panels
// are independent backends, so the same user name on panel A must not block
// panel B (a global map caused cross-panel mis-claiming).
func (p *PanelClient) claimMiniMaxUser(user string) bool {
	user = strings.TrimSpace(user)
	if user == "" {
		return false
	}
	p.claimedMu.Lock()
	defer p.claimedMu.Unlock()
	if p.claimedUsers == nil {
		p.claimedUsers = map[string]struct{}{}
	}
	if _, exists := p.claimedUsers[user]; exists {
		return false
	}
	p.claimedUsers[user] = struct{}{}
	return true
}

func (p *PanelClient) WaitForBinding(ctx context.Context, signInID string, beforeUsers []string, timeout time.Duration) (*SignInPollResp, string, error) {
	before := map[string]struct{}{}
	for _, user := range beforeUsers {
		before[user] = struct{}{}
	}
	deadline := time.Now().Add(timeout)
	var latest *SignInPollResp
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return nil, "", err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/api/signin/"+url.PathEscape(signInID), nil)
		if err != nil {
			return nil, "", err
		}
		if resp, err := p.http.Do(req); err == nil {
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode == 200 {
				var out SignInPollResp
				if json.Unmarshal(body, &out) == nil {
					latest = &out
					if out.State == "error" || out.State == "expired" || out.State == "failed" {
						return &out, "", fmt.Errorf("面板 sign-in 终态异常: %s (%s)", out.State, out.Error)
					}
					if out.State == "done" && strings.TrimSpace(out.User) != "" && p.claimMiniMaxUser(out.User) {
						return &out, out.User, nil
					}
				}
			}
		}
		users, err := p.ListMiniMaxUsers(ctx)
		if err == nil {
			for _, user := range users {
				if _, known := before[user]; known {
					continue
				}
				if p.claimMiniMaxUser(user) {
					if latest == nil {
						latest = &SignInPollResp{ID: signInID, State: "done", User: user}
					} else {
						latest.User = user
						latest.State = "done"
					}
					return latest, user, nil
				}
			}
		}
		select {
		case <-ctx.Done():
			return nil, "", ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	if latest != nil && latest.State == "done" && latest.User != "" {
		return latest, latest.User, nil
	}
	return latest, "", fmt.Errorf("轮询 /api/signin/%s 超时", signInID)
}
