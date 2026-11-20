package main

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const minimaxOrigin = "https://account.minimax.io"

// agentOrigin serves the MiniMax Code (agent) web app, which owns the daily check-in API.
const agentOrigin = "https://agent.minimax.io"

const minimaxRSAPubPEM = `-----BEGIN PUBLIC KEY-----
MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQDF5ndG2/UB4L5tbvQaNLHSoBTW
DKbrNBuOmUIP23eCmC2ELMx3kppEikxTp5cV8NxUZl6ii+KLwKugioAXApzypHXb
gXbq13kTKA7OCA1xtAoMdH9cltjBiFAUJlgmVjr0MuJCknhVAjWLjCVRHege+Atl
gkUBUeGa9O+cWcPEwQIDAQAB
-----END PUBLIC KEY-----`

type MiniMaxClient struct {
	http          *http.Client
	cookies       map[string]string
	uuid          string
	deviceID      string
	realUserID    string
	timezoneID    string
	signatureSalt string
	yySuffix      string
}


func NewMiniMaxClient(dialer *ProxyDialer, bridgeAddr, signatureSalt, yySuffix string) *MiniMaxClient {
	var client *http.Client
	if dialer != nil && dialer.Enabled() {
		if strings.TrimSpace(bridgeAddr) != "" {
			client = dialer.HTTPClientViaBridge(bridgeAddr, 25*time.Second)
		} else {
			client = dialer.HTTPClient(25 * time.Second)
		}
	} else {
		client = &http.Client{Timeout: 25 * time.Second}
	}
	return &MiniMaxClient{
		http:          client,
		cookies:       map[string]string{},
		realUserID:    "0",
		signatureSalt: signatureSalt,
		yySuffix:      yySuffix,
	}
}

func (m *MiniMaxClient) SetCookies(c map[string]string) {
	for k, v := range c {
		if k != "" && v != "" {
			m.cookies[k] = v
		}
	}
}

func (m *MiniMaxClient) CookieSnapshot() map[string]string {
	out := make(map[string]string, len(m.cookies))
	for k, v := range m.cookies {
		out[k] = v
	}
	return out
}

func (m *MiniMaxClient) SetTimezone(id string) {
	m.timezoneID = strings.TrimSpace(id)
}

func (m *MiniMaxClient) cookieHeader() string {
	if len(m.cookies) == 0 {
		return ""
	}
	parts := make([]string, 0, len(m.cookies))
	for k, v := range m.cookies {
		parts = append(parts, k+"="+v)
	}
	return strings.Join(parts, "; ")
}

func (m *MiniMaxClient) saveRespCookies(resp *http.Response) {
	for _, sc := range resp.Cookies() {
		if sc.Name != "" {
			m.cookies[sc.Name] = sc.Value
		}
	}
}

func md5Hex(s string) string {
	sum := md5.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}

func randomUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func encryptMiniMaxPassword(plain string) (string, error) {
	block, _ := pem.Decode([]byte(minimaxRSAPubPEM))
	if block == nil {
		return "", errors.New("解析 MiniMax RSA 公钥失败")
	}
	pubAny, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return "", err
	}
	pub, ok := pubAny.(*rsa.PublicKey)
	if !ok {
		return "", errors.New("非 RSA 公钥")
	}
	enc, err := rsa.EncryptPKCS1v15(rand.Reader, pub, []byte(plain))
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(enc), nil
}

func (m *MiniMaxClient) signedRequest(ctx context.Context, method, reqPath string, bodyObj any) (int, map[string]any, string, error) {
	return m.signedRequestOn(ctx, minimaxOrigin, "/unified-login", method, reqPath, nil, bodyObj)
}

func (m *MiniMaxClient) signedRequestOn(ctx context.Context, origin, refererPath, method, reqPath string, extraParams map[string]string, bodyObj any) (int, map[string]any, string, error) {
	unixMs := time.Now().UnixMilli()
	ts := unixMs / 1000
	_, offsetSec := time.Now().Zone()
	tzOffset := -(offsetSec / 60)

	var payload string
	if strings.ToUpper(method) == http.MethodPost {
		b, err := json.Marshal(bodyObj)
		if err != nil {
			return 0, nil, "", err
		}
		payload = string(b)
	}

	q := url.Values{}
	q.Set("device_platform", "web")
	q.Set("biz_id", "3")
	q.Set("app_id", "3001")
	q.Set("version_code", "22201")
	q.Set("unix", strconv.FormatInt(unixMs, 10))
	q.Set("timezone_offset", strconv.Itoa(tzOffset))
	q.Set("sys_language", "en")
	q.Set("lang", "en")
	q.Set("uuid", m.uuid)
	if m.deviceID != "" {
		q.Set("device_id", m.deviceID)
	} else {
		q.Set("device_id", "0")
	}
	q.Set("os_name", "Windows")
	q.Set("browser_name", "Chrome")
	q.Set("device_memory", "8")
	q.Set("cpu_core_num", "8")
	q.Set("browser_language", "en-US")
	q.Set("browser_platform", "Win32")
	q.Set("screen_width", "3440")
	q.Set("screen_height", "1440")
	if m.realUserID != "" {
		q.Set("user_id", m.realUserID)
	} else {
		q.Set("user_id", "0")
	}
	q.Set("client", "web")
	for k, v := range extraParams {
		q.Set(k, v)
	}

	fullPath := reqPath + "?" + q.Encode()
	yyBody := "{}"
	if strings.ToUpper(method) == http.MethodPost {
		yyBody = payload
	}
	yyInput := url.QueryEscape(fullPath) + "_" + yyBody + md5Hex(strconv.FormatInt(unixMs, 10)) + m.yySuffix
	xSig := md5Hex(strconv.FormatInt(ts, 10) + m.signatureSalt + payload)
	yy := md5Hex(yyInput)

	var bodyReader io.Reader
	if payload != "" {
		bodyReader = strings.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, origin+fullPath, bodyReader)
	if err != nil {
		return 0, nil, "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/157.0.8103.51 Safari/537.36")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Origin", origin)
	req.Header.Set("Referer", origin+refererPath)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-timestamp", strconv.FormatInt(ts, 10))
	req.Header.Set("x-signature", xSig)
	req.Header.Set("yy", yy)
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
	raw := string(rawBytes)
	var parsed map[string]any
	_ = json.Unmarshal(rawBytes, &parsed)
	return resp.StatusCode, parsed, raw, nil
}

// SigninStatus 读取 MiniMax Code 每日签到面板（agent.minimax.io）。
func (m *MiniMaxClient) SigninStatus(ctx context.Context) (int, map[string]any, string, error) {
	return m.signedRequestOn(ctx, agentOrigin, "/", http.MethodGet,
		"/minimax-cloud/api/v1/signin/status",
		map[string]string{"timezone_id": "Asia/Shanghai", "region": "en"}, nil)
}

// ClaimSignin 执行每日签到领取积分。
func (m *MiniMaxClient) ClaimSignin(ctx context.Context) (int, map[string]any, string, error) {
	return m.signedRequestOn(ctx, agentOrigin, "/", http.MethodPost,
		"/minimax-cloud/api/v1/signin/claim",
		map[string]string{"timezone_id": "Asia/Shanghai", "region": "en"}, map[string]any{})
}

func (m *MiniMaxClient) plainRequest(ctx context.Context, method, fullOrPath string, bodyObj any) (int, map[string]any, string, error) {
	target := fullOrPath
	if !strings.HasPrefix(target, "http") {
		target = minimaxOrigin + target
	}
	var bodyReader io.Reader
	if bodyObj != nil {
		b, err := json.Marshal(bodyObj)
		if err != nil {
			return 0, nil, "", err
		}
		bodyReader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, bodyReader)
	if err != nil {
		return 0, nil, "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/157.0.8103.51 Safari/537.36")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Origin", minimaxOrigin)
	req.Header.Set("Referer", minimaxOrigin+"/unified-login")
	req.Header.Set("Cache-Control", "no-store")
	if bodyObj != nil {
		req.Header.Set("Content-Type", "application/json")
	}
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
	raw := string(rawBytes)
	var parsed map[string]any
	_ = json.Unmarshal(rawBytes, &parsed)
	return resp.StatusCode, parsed, raw, nil
}

func (m *MiniMaxClient) RegisterDevice(ctx context.Context) error {
	if m.uuid == "" {
		m.uuid = randomUUID()
	}
	status, parsed, raw, err := m.signedRequest(ctx, http.MethodPost, "/v1/api/user/device/register", map[string]any{
		"uuid": m.uuid,
	})
	if err != nil {
		return err
	}
	if status >= 400 {
		return fmt.Errorf("register-device HTTP %d: %s", status, raw)
	}
	if data, ok := parsed["data"].(map[string]any); ok {
		if id, ok := data["deviceIDStr"].(string); ok && id != "" {
			m.deviceID = id
		} else if id, ok := data["deviceIdStr"].(string); ok && id != "" {
			m.deviceID = id
		}
	}
	if m.deviceID == "" {
		return fmt.Errorf("register-device 未返回 deviceIDStr: %s", raw)
	}
	return nil
}

func extractStatusCode(parsed map[string]any) (int, string) {
	si, ok := parsed["statusInfo"].(map[string]any)
	if ok {
		code := -1
		if f, ok := si["code"].(float64); ok {
			code = int(f)
		}
		msg, _ := si["message"].(string)
		return code, msg
	}
	// Some oauth2/login responses use {"code":0,"msg":"ok","data":...}
	if f, ok := parsed["code"].(float64); ok {
		msg, _ := parsed["msg"].(string)
		if msg == "" {
			msg, _ = parsed["message"].(string)
		}
		return int(f), msg
	}
	return -1, ""
}

func (m *MiniMaxClient) CheckAccount(ctx context.Context, email, loginRedirect string) (int, string, error) {
	status, parsed, raw, err := m.signedRequest(ctx, http.MethodPost, "/oauth2/login", map[string]any{
		"loginType":      "20",
		"email":          email,
		"authToken":      nil,
		"deviceID":       m.deviceID,
		"login_redirect": loginRedirect,
	})
	if err != nil {
		return -1, "", err
	}
	if status >= 500 {
		return -1, "", fmt.Errorf("check-account HTTP %d: %s", status, raw)
	}
	code, msg := extractStatusCode(parsed)
	return code, msg, nil
}

func (m *MiniMaxClient) SendCode(ctx context.Context, email, ticket string) error {
	status, parsed, raw, err := m.signedRequest(ctx, http.MethodPost, "/v1/api/user/login/sms/send", map[string]any{
		"email":       email,
		"phone":       "",
		"randStr":     "",
		"ticket":      ticket,
		"captchaType": "recaptcha",
	})
	if err != nil {
		return err
	}
	code, msg := extractStatusCode(parsed)
	if status >= 400 || code != 0 {
		return fmt.Errorf("send-code: %d %s (%s)", code, msg, raw)
	}
	return nil
}

func (m *MiniMaxClient) CreatePasswordLogin(ctx context.Context, email, password, emailCode, loginRedirect string) error {
	encPwd, err := encryptMiniMaxPassword(password)
	if err != nil {
		return err
	}
	status, parsed, raw, err := m.signedRequest(ctx, http.MethodPost, "/oauth2/login", map[string]any{
		"loginType":      "20",
		"email":          email,
		"authToken":      encPwd,
		"code":           emailCode,
		"deviceID":       m.deviceID,
		"login_redirect": loginRedirect,
	})
	if err != nil {
		return err
	}
	code, msg := extractStatusCode(parsed)
	if status >= 400 || code != 0 {
		return fmt.Errorf("create-password login 失败: %d %s (%s)", code, msg, raw)
	}
	if data, ok := parsed["data"].(map[string]any); ok {
		if uid, ok := data["realUserID"].(string); ok && uid != "" {
			m.realUserID = uid
		} else if uid, ok := data["str_user_id"].(string); ok && uid != "" {
			m.realUserID = uid
		} else if uid, ok := data["user_id"].(float64); ok && uid != 0 {
			m.realUserID = strconv.FormatInt(int64(uid), 10)
		}
	}
	return nil
}

// CodeOnlyLogin 已有密码账号的纯验证码登录（loginType=21，不带 authToken）。
// 用于号池残留账号：建密接口 1200058 表明该邮箱已有密码，此时同一封验证码可直接走 21 号登录，
// 救回"密码已设但流程中途失败被放回可用池"的账号。
func (m *MiniMaxClient) CodeOnlyLogin(ctx context.Context, email, emailCode, loginRedirect string) error {
	status, parsed, raw, err := m.signedRequest(ctx, http.MethodPost, "/oauth2/login", map[string]any{
		"loginType":      "21",
		"email":          email,
		"authToken":      nil,
		"code":           emailCode,
		"deviceID":       m.deviceID,
		"login_redirect": loginRedirect,
	})
	if err != nil {
		return err
	}
	code, msg := extractStatusCode(parsed)
	if status >= 400 || code != 0 {
		return fmt.Errorf("code-only login 失败: %d %s (%s)", code, msg, raw)
	}
	if data, ok := parsed["data"].(map[string]any); ok {
		if uid, ok := data["realUserID"].(string); ok && uid != "" {
			m.realUserID = uid
		} else if uid, ok := data["str_user_id"].(string); ok && uid != "" {
			m.realUserID = uid
		} else if uid, ok := data["user_id"].(float64); ok && uid != 0 {
			m.realUserID = strconv.FormatInt(int64(uid), 10)
		}
	}
	return nil
}

func (m *MiniMaxClient) AuthorizeDevice(ctx context.Context, userCode string) error {
	q := url.Values{}
	q.Set("user_code", userCode)
	q.Set("app_id", "3001")
	status, parsed, raw, err := m.plainRequest(ctx, http.MethodGet, "/oauth2/device/authorize?"+q.Encode(), nil)
	if err != nil {
		return err
	}
	if status >= 400 {
		return fmt.Errorf("authorize status HTTP %d: %s", status, raw)
	}
	st, _ := parsed["status"].(string)
	if st == "approved" {
		return nil
	}
	if st != "pending" {
		return fmt.Errorf("设备码状态非 pending: %s (%s)", st, raw)
	}
	groups, _ := parsed["groups"].([]any)
	var groupID string
	for _, gAny := range groups {
		g, ok := gAny.(map[string]any)
		if !ok {
			continue
		}
		isDef, _ := g["is_default"].(bool)
		canAuth, _ := g["can_authorize"].(bool)
		if isDef && canAuth {
			groupID, _ = g["group_id"].(string)
			break
		}
	}
	if groupID == "" {
		return fmt.Errorf("未找到可授权的默认 group_id: %s", raw)
	}
	postStatus, postParsed, postRaw, err := m.plainRequest(ctx, http.MethodPost, "/oauth2/device/authorize", map[string]any{
		"user_code": userCode,
		"decision":  "approve",
		"group_id":  groupID,
	})
	if err != nil {
		return err
	}
	if postStatus >= 400 {
		return fmt.Errorf("POST /oauth2/device/authorize HTTP %d: %s", postStatus, postRaw)
	}
	if finalSt, _ := postParsed["status"].(string); finalSt != "approved" {
		// Re-check once
		_, chkParsed, _, _ := m.plainRequest(ctx, http.MethodGet, "/oauth2/device/authorize?"+q.Encode(), nil)
		if s, _ := chkParsed["status"].(string); s != "approved" {
			return fmt.Errorf("设备授权未变成 approved: %s", postRaw)
		}
	}
	return nil
}
