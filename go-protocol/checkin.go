package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// MiniMax Code 的公开客户端与签到接口参数（与 magpie 插件一致）。
const (
	mcodeClientID  = "mcode-public"
	mcodeScope     = "agent.default"
	mcodeAudience  = "agent-backend"
	mcodeDeviceURN = "urn:ietf:params:oauth:grant-type:device_code"
	mcodeDesktopUA = "MiniMaxCode"
)

type DeviceFlow struct {
	DeviceCode string
	UserCode   string
	Verifier   string
	Interval   time.Duration
	Deadline   time.Time
}

type CheckinResult struct {
	Points   int
	Claimed  bool
	Streak   int
	Days     int
	RawClaim string
}

func randomBase64URL(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func (m *MiniMaxClient) formPost(ctx context.Context, target string, fields map[string]string) (int, map[string]any, string, error) {
	form := url.Values{}
	for k, v := range fields {
		form.Set(k, v)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, strings.NewReader(form.Encode()))
	if err != nil {
		return 0, nil, "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", mcodeDesktopUA)
	if ch := m.cookieHeader(); ch != "" {
		req.Header.Set("Cookie", ch)
	}
	resp, err := m.http.Do(req)
	if err != nil {
		return 0, nil, "", err
	}
	defer resp.Body.Close()
	m.saveRespCookies(resp)
	rawBytes, _ := io.ReadAll(resp.Body)
	var parsed map[string]any
	_ = json.Unmarshal(rawBytes, &parsed)
	return resp.StatusCode, parsed, string(rawBytes), nil
}

// StartOwnDeviceFlow 用 MiniMax Code 的公开客户端发起一次独立设备码授权（PKCE）。
func (m *MiniMaxClient) StartOwnDeviceFlow(ctx context.Context) (*DeviceFlow, error) {
	verifier := randomBase64URL(32)
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	status, parsed, raw, err := m.formPost(ctx, minimaxOrigin+"/oauth2/device/code", map[string]string{
		"client_id":             mcodeClientID,
		"scope":                 mcodeScope,
		"audience":              mcodeAudience,
		"code_challenge":        challenge,
		"code_challenge_method": "S256",
	})
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("设备码申请失败 HTTP %d: %s", status, trimForLog(raw))
	}
	flow := &DeviceFlow{Verifier: verifier}
	flow.UserCode, _ = parsed["user_code"].(string)
	flow.DeviceCode, _ = parsed["device_code"].(string)
	if flow.UserCode == "" {
		return nil, fmt.Errorf("设备码响应缺少 user_code: %s", trimForLog(raw))
	}
	if v, ok := parsed["interval"].(float64); ok && v > 0 {
		flow.Interval = time.Duration(v) * time.Second
	}
	if flow.Interval <= 0 {
		flow.Interval = 5 * time.Second
	}
	ttl := 600 * time.Second
	if v, ok := parsed["expires_in"].(float64); ok && v > 0 {
		ttl = time.Duration(v) * time.Second
	}
	flow.Deadline = time.Now().Add(ttl)
	return flow, nil
}

// ExchangeDeviceToken 在用户码被批准后换取 access/refresh token。
func (m *MiniMaxClient) ExchangeDeviceToken(ctx context.Context, flow *DeviceFlow) (string, string, error) {
	wait := flow.Interval
	for time.Now().Before(flow.Deadline) {
		fields := map[string]string{
			"grant_type":    mcodeDeviceURN,
			"client_id":     mcodeClientID,
			"code_verifier": flow.Verifier,
		}
		if flow.DeviceCode != "" {
			fields["device_code"] = flow.DeviceCode
		} else {
			fields["user_code"] = flow.UserCode
		}
		status, parsed, raw, err := m.formPost(ctx, minimaxOrigin+"/oauth2/token", fields)
		if err != nil {
			return "", "", err
		}
		if errCode, _ := parsed["error"].(string); errCode == "authorization_pending" || errCode == "slow_down" {
			time.Sleep(wait)
			continue
		}
		if errCode, _ := parsed["error"].(string); errCode == "access_denied" {
			return "", "", fmt.Errorf("设备授权被拒绝")
		}
		if st, _ := parsed["status"].(string); st == "pending" || st == "slow_down" {
			time.Sleep(wait)
			continue
		}
		if status >= 400 {
			return "", "", fmt.Errorf("令牌交换失败 HTTP %d: %s", status, trimForLog(raw))
		}
		access, _ := parsed["access_token"].(string)
		if access == "" {
			return "", "", fmt.Errorf("令牌响应缺少 access_token: %s", trimForLog(raw))
		}
		refresh, _ := parsed["refresh_token"].(string)
		return access, refresh, nil
	}
	return "", "", fmt.Errorf("设备码授权超时")
}

// mcodeSigned 按 MiniMax Code 桌面端的签名规则调用 agent 侧账号接口。
// RefreshAccessToken 用 refresh_token 换新的 access token（每日补签不需要再收验证码）。
func (m *MiniMaxClient) RefreshAccessToken(ctx context.Context, refresh string) (string, string, error) {
	status, parsed, raw, err := m.formPost(ctx, minimaxOrigin+"/oauth2/token", map[string]string{
		"grant_type":    "refresh_token",
		"refresh_token": refresh,
		"client_id":     mcodeClientID,
		"scope":         mcodeScope,
		"audience":      mcodeAudience,
	})
	if err != nil {
		return "", "", err
	}
	if status >= 400 {
		return "", "", fmt.Errorf("刷新令牌失败 HTTP %d: %s", status, trimForLog(raw))
	}
	access, _ := parsed["access_token"].(string)
	if access == "" {
		return "", "", fmt.Errorf("刷新令牌响应缺少 access_token: %s", trimForLog(raw))
	}
	newRefresh, _ := parsed["refresh_token"].(string)
	if newRefresh == "" {
		newRefresh = refresh
	}
	return access, newRefresh, nil
}

func (m *MiniMaxClient) mcodeSigned(ctx context.Context, method, path, access, userID string, bodyObj any) (int, map[string]any, string, error) {
	now := time.Now()
	unixMs := now.UnixMilli()
	ts := unixMs / 1000
	_, offsetSec := now.Zone()

	var payload string
	if strings.EqualFold(method, http.MethodPost) {
		b, err := json.Marshal(bodyObj)
		if err != nil {
			return 0, nil, "", err
		}
		payload = string(b)
	}

	q := url.Values{}
	q.Set("device_platform", "web")
	q.Set("is_desktop", "1")
	q.Set("desktop_version", "0.6.12")
	q.Set("biz_id", "3")
	q.Set("app_id", "3001")
	q.Set("version_code", "22201")
	q.Set("unix", strconv.FormatInt(unixMs, 10))
	q.Set("timezone_offset", strconv.Itoa(-(offsetSec / 60)))
	q.Set("sys_language", "en")
	q.Set("lang", "en")
	q.Set("device_id", "0")
	q.Set("os_name", "Windows")
	q.Set("browser_name", "mcode")
	if strings.TrimSpace(userID) == "" {
		q.Set("user_id", "0")
	} else {
		q.Set("user_id", userID)
	}
	q.Set("client", "mcode")
	tzID := strings.TrimSpace(m.timezoneID)
	if tzID == "" {
		tzID = "Asia/Shanghai"
	}
	q.Set("timezone_id", tzID)

	fullPath := path + "?" + q.Encode()
	yyBody := "{}"
	if payload != "" {
		yyBody = payload
	}
	yy := md5Hex(url.QueryEscape(fullPath) + "_" + yyBody + md5Hex(strconv.FormatInt(unixMs, 10)) + m.yySuffix)
	xSig := md5Hex(strconv.FormatInt(ts, 10) + m.signatureSalt + payload)

	var reader io.Reader
	if payload != "" {
		reader = strings.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, agentOrigin+fullPath, reader)
	if err != nil {
		return 0, nil, "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", mcodeDesktopUA)
	req.Header.Set("Authorization", "Bearer "+access)
	req.Header.Set("yy", yy)
	req.Header.Set("x-timestamp", strconv.FormatInt(ts, 10))
	req.Header.Set("x-signature", xSig)
	resp, err := m.http.Do(req)
	if err != nil {
		return 0, nil, "", err
	}
	defer resp.Body.Close()
	rawBytes, _ := io.ReadAll(resp.Body)
	var parsed map[string]any
	_ = json.Unmarshal(rawBytes, &parsed)
	return resp.StatusCode, parsed, string(rawBytes), nil
}

// MCodeUserID 读取该 token 对应的真实用户 ID（签到请求必须带）。
func (m *MiniMaxClient) MCodeUserID(ctx context.Context, access string) string {
	status, parsed, _, err := m.mcodeSigned(ctx, http.MethodGet, "/v1/api/user/info", access, m.realUserID, nil)
	if err != nil || status != 200 {
		return m.realUserID
	}
	pick := func(o map[string]any) string {
		for _, k := range []string{"realUserID", "real_user_id"} {
			if v, ok := o[k].(string); ok && v != "" {
				return v
			}
		}
		return ""
	}
	if d, ok := parsed["data"].(map[string]any); ok {
		for _, k := range []string{"userInfo", "user_info"} {
			if u, ok := d[k].(map[string]any); ok {
				if id := pick(u); id != "" {
					return id
				}
			}
		}
	}
	for _, k := range []string{"userInfo", "user_info"} {
		if u, ok := parsed[k].(map[string]any); ok {
			if id := pick(u); id != "" {
				return id
			}
		}
	}
	return m.realUserID
}

// CheckinWithToken 读取签到面板并领取当天积分。
func (m *MiniMaxClient) CheckinWithToken(ctx context.Context, access, userID string) (CheckinResult, error) {
	res := CheckinResult{}
	status, parsed, raw, err := m.mcodeSigned(ctx, http.MethodGet, "/minimax-cloud/api/v1/signin/status", access, userID, nil)
	if err != nil {
		return res, err
	}
	if status != 200 {
		return res, fmt.Errorf("签到状态 HTTP %d: %s", status, trimForLog(raw))
	}
	if msg, bad := baseRespError(parsed); bad {
		return res, fmt.Errorf("签到状态被拒: %s", msg)
	}
	if data, ok := parsed["data"].(map[string]any); ok {
		if days, ok := data["days"].([]any); ok {
			res.Days = len(days)
			for _, dAny := range days {
				d, ok := dAny.(map[string]any)
				if !ok {
					continue
				}
				isToday, _ := d["is_today"].(bool)
				if !isToday {
					continue
				}
				if st, ok := d["status"].(float64); ok && st == 3 {
					res.Claimed = false
					if p, ok := d["points"].(float64); ok {
						res.Points = int(p)
					}
					return res, nil
				}
			}
		}
	}

	claimStatus, claimParsed, claimRaw, err := m.mcodeSigned(ctx, http.MethodPost, "/minimax-cloud/api/v1/signin/claim", access, userID, map[string]any{})
	if err != nil {
		return res, err
	}
	res.RawClaim = trimForLog(claimRaw)
	if claimStatus != 200 {
		return res, fmt.Errorf("签到领取 HTTP %d: %s", claimStatus, trimForLog(claimRaw))
	}
	if msg, bad := baseRespError(claimParsed); bad {
		return res, fmt.Errorf("签到领取被拒: %s", msg)
	}
	if data, ok := claimParsed["data"].(map[string]any); ok {
		if cr, ok := data["claim_result"].(float64); ok {
			res.Claimed = int(cr) == 1
		}
		if p, ok := data["points"].(float64); ok {
			res.Points = int(p)
		}
	}
	return res, nil
}

// baseRespError 把响应体里的业务错误（base_resp.status_code）提取出来。
func baseRespError(parsed map[string]any) (string, bool) {
	br, ok := parsed["base_resp"].(map[string]any)
	if !ok {
		return "", false
	}
	code, ok := br["status_code"].(float64)
	if !ok || code == 0 {
		return "", false
	}
	msg, _ := br["status_msg"].(string)
	if msg == "" {
		msg = "status_code=" + strconv.Itoa(int(code))
	}
	return fmt.Sprintf("%s (%d)", msg, int(code)), true
}
