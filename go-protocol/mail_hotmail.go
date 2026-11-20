package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	hotmailTokenURL = "https://login.microsoftonline.com/consumers/oauth2/v2.0/token"
	hotmailGraphAPI = "https://graph.microsoft.com/v1.0"
	hotmailScope    = "https://graph.microsoft.com/Mail.ReadWrite offline_access"
)

type HotmailAccount struct {
	Email        string `json:"email"`
	Password     string `json:"password"`
	ClientID     string `json:"clientId"`
	RefreshToken string `json:"refreshToken"`
	Status       string `json:"status"` // available | reserved | used | skipped
	FailCount    int    `json:"failCount,omitempty"`
	LastError    string `json:"lastError,omitempty"`
	UpdatedAt    string `json:"updatedAt,omitempty"`
}

type HotmailPool struct {
	Version   int              `json:"version"`
	UpdatedAt string           `json:"updatedAt"`
	Accounts  []HotmailAccount `json:"accounts"`
}

type HotmailPoolStats struct {
	Total     int `json:"total"`
	Available int `json:"available"`
	Reserved  int `json:"reserved"`
	Used      int `json:"used"`
	Skipped   int `json:"skipped"`
}

var uuidRe = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func LoadHotmailPool(path string) (*HotmailPool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &HotmailPool{Version: 1, Accounts: []HotmailAccount{}}, nil
	}
	if err != nil {
		return nil, err
	}
	var p HotmailPool
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	if p.Accounts == nil {
		p.Accounts = []HotmailAccount{}
	}
	return &p, nil
}

func SaveHotmailPool(path string, p *HotmailPool) error {
	p.Version = 1
	p.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// HotmailPoolStore serializes pool claim/finalize across concurrent workers.
type HotmailPoolStore struct {
	mu   sync.Mutex
	path string
}

var ErrHotmailPoolExhausted = errors.New("Hotmail 账号池已无可用邮箱")

// RecoverStaleHotmailReservations 把上次进程被强制结束遗留的 reserved 账号放回可用池。
func RecoverStaleHotmailReservations(path string, maxAge time.Duration) (int, error) {
	pool, err := LoadHotmailPool(path)
	if err != nil {
		return 0, err
	}
	now := time.Now()
	recovered := 0
	for i := range pool.Accounts {
		if strings.TrimSpace(pool.Accounts[i].Status) != "reserved" {
			continue
		}
		updatedAt, parseErr := time.Parse(time.RFC3339, pool.Accounts[i].UpdatedAt)
		if parseErr != nil || now.Sub(updatedAt) >= maxAge {
			pool.Accounts[i].Status = "available"
			pool.Accounts[i].LastError = "recovered from interrupted run"
			pool.Accounts[i].UpdatedAt = now.UTC().Format(time.RFC3339)
			recovered++
		}
	}
	if recovered > 0 {
		if err := SaveHotmailPool(path, pool); err != nil {
			return 0, err
		}
	}
	return recovered, nil
}

// RecycleReserved 把上次进程异常退出时残留的 reserved 账号放回可用池。
func RecycleReserved(path string) (int, error) {
	pool, err := LoadHotmailPool(path)
	if err != nil {
		return 0, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	n := 0
	for i := range pool.Accounts {
		if pool.Accounts[i].Status == "reserved" {
			pool.Accounts[i].Status = "available"
			pool.Accounts[i].UpdatedAt = now
			n++
		}
	}
	if n == 0 {
		return 0, nil
	}
	return n, SaveHotmailPool(path, pool)
}

func NewHotmailPoolStore(path string) *HotmailPoolStore {
	return &HotmailPoolStore{path: path}
}

// Claim reserves one available account. Returned index is valid for Finalize.
func (s *HotmailPoolStore) Claim() (*HotmailAccount, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	pool, err := LoadHotmailPool(s.path)
	if err != nil {
		return nil, -1, err
	}
	best := -1
	var bestAt time.Time
	for i := range pool.Accounts {
		status := strings.TrimSpace(pool.Accounts[i].Status)
		if status != "" && status != "available" {
			continue
		}
		at, _ := time.Parse(time.RFC3339, pool.Accounts[i].UpdatedAt)
		if best < 0 || at.Before(bestAt) {
			best = i
			bestAt = at
		}
	}
	if best < 0 {
		return nil, -1, ErrHotmailPoolExhausted
	}
	pool.Accounts[best].Status = "reserved"
	acc := pool.Accounts[best]
	if err := SaveHotmailPool(s.path, pool); err != nil {
		return nil, -1, err
	}
	return &acc, best, nil
}

func (s *HotmailPoolStore) Finalize(index int, email, status, lastErr, refreshToken string) {
	if index < 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	pool, err := LoadHotmailPool(s.path)
	if err != nil || index >= len(pool.Accounts) {
		return
	}
	if email != "" && !strings.EqualFold(pool.Accounts[index].Email, email) {
		found := -1
		for i := range pool.Accounts {
			if strings.EqualFold(pool.Accounts[i].Email, email) {
				found = i
				break
			}
		}
		if found < 0 {
			return
		}
		index = found
	}
	if status == "available" {
		pool.Accounts[index].FailCount++
		// 可恢复失败（IP 风控 1200058、人机 1200063、验证码过期 1200059、网络抖动等）：
		// 保持 available，UpdatedAt 刷新会自动排到队尾，下一轮重新尝试。
		// 仅当连续 10 轮全失败（确认真坏号，如令牌失效残留）才永久 skipped，避免无限空转。
		if pool.Accounts[index].FailCount >= 10 {
			status = "skipped"
			lastErr = "连续 10 轮失败，自动跳过: " + lastErr
		}
	}
	if status == "used" {
		pool.Accounts[index].FailCount = 0
	}
	pool.Accounts[index].Status = status
	pool.Accounts[index].LastError = lastErr
	if refreshToken != "" {
		pool.Accounts[index].RefreshToken = refreshToken
	}
	pool.Accounts[index].UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	_ = SaveHotmailPool(s.path, pool)
}

func (s *HotmailPoolStore) Stats() (HotmailPoolStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	pool, err := LoadHotmailPool(s.path)
	if err != nil {
		return HotmailPoolStats{}, err
	}
	return pool.Stats(), nil
}

func (p *HotmailPool) Stats() HotmailPoolStats {
	var st HotmailPoolStats
	if p == nil {
		return st
	}
	st.Total = len(p.Accounts)
	for _, a := range p.Accounts {
		switch a.Status {
		case "available":
			st.Available++
		case "reserved":
			st.Reserved++
		case "used":
			st.Used++
		case "skipped":
			st.Skipped++
		default:
			st.Available++
		}
	}
	return st
}

func ImportHotmailRaw(path, raw string) (HotmailPoolStats, int, error) {
	pool, err := LoadHotmailPool(path)
	if err != nil {
		return HotmailPoolStats{}, 0, err
	}
	existing := map[string]int{}
	for i, a := range pool.Accounts {
		existing[strings.ToLower(a.Email)] = i
	}
	added := 0
	raw = strings.TrimPrefix(raw, "\ufeff")
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		var parts []string
		if strings.Contains(line, "\t") {
			for _, f := range strings.Split(line, "\t") {
				if t := strings.TrimSpace(f); t != "" {
					parts = append(parts, t)
				}
			}
		} else {
			for _, f := range strings.Split(line, "----") {
				parts = append(parts, strings.TrimSpace(f))
			}
		}
		if len(parts) < 4 {
			continue
		}
		email := strings.ToLower(parts[0])
		pwd := parts[1]
		cid := parts[2]
		rt := strings.Join(parts[3:], "----")
		if !strings.Contains(email, "@") || pwd == "" || cid == "" || rt == "" {
			continue
		}
		if !uuidRe.MatchString(cid) && len(cid) < 8 {
			continue
		}
		now := time.Now().UTC().Format(time.RFC3339)
		if idx, ok := existing[email]; ok {
			pool.Accounts[idx].Password = pwd
			pool.Accounts[idx].ClientID = cid
			pool.Accounts[idx].RefreshToken = rt
			if pool.Accounts[idx].Status == "" || pool.Accounts[idx].Status == "skipped" {
				pool.Accounts[idx].Status = "available"
			}
			pool.Accounts[idx].UpdatedAt = now
		} else {
			existing[email] = len(pool.Accounts)
			pool.Accounts = append(pool.Accounts, HotmailAccount{
				Email:        email,
				Password:     pwd,
				ClientID:     cid,
				RefreshToken: rt,
				Status:       "available",
				UpdatedAt:    now,
			})
			added++
		}
	}
	if err := SaveHotmailPool(path, pool); err != nil {
		return HotmailPoolStats{}, 0, err
	}
	return pool.Stats(), added, nil
}

func RefreshHotmailToken(ctx context.Context, client *http.Client, acc *HotmailAccount) (string, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		token, err := refreshHotmailTokenOnce(ctx, client, acc)
		if err == nil {
			return token, nil
		}
		lastErr = err
		msg := strings.ToLower(err.Error())
		if !strings.Contains(msg, "timeout") && !strings.Contains(msg, "tls") && !strings.Contains(msg, "eof") && !strings.Contains(msg, "reset") {
			return "", err
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(time.Duration(attempt+1) * 2 * time.Second):
		}
	}
	return "", lastErr
}

func refreshHotmailTokenOnce(ctx context.Context, client *http.Client, acc *HotmailAccount) (string, error) {
	form := url.Values{}
	form.Set("client_id", acc.ClientID)
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", acc.RefreshToken)
	form.Set("scope", hotmailScope)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, hotmailTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var parsed struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		Error        string `json:"error"`
		ErrorDesc    string `json:"error_description"`
	}
	_ = json.Unmarshal(raw, &parsed)
	if resp.StatusCode >= 400 || parsed.AccessToken == "" {
		msg := parsed.ErrorDesc
		if msg == "" {
			msg = string(raw)
		}
		return "", fmt.Errorf("微软令牌刷新失败: %s", msg)
	}
	if parsed.RefreshToken != "" {
		acc.RefreshToken = parsed.RefreshToken
	}
	return parsed.AccessToken, nil
}

// PollHotmailCode waits for a MiniMax verification mail that arrived after notBefore.
// Mailboxes reused across attempts often still hold an older code; accepting it would
// always fail with 1200059 (code incorrect or expired).
func PollHotmailCode(ctx context.Context, client *http.Client, acc *HotmailAccount, notBefore time.Time, timeout time.Duration) (string, error) {
	accessToken, err := RefreshHotmailToken(ctx, client, acc)
	if err != nil {
		return "", err
	}
	deadline := time.Now().Add(timeout)
	notBeforeUTC := notBefore.UTC().Add(-2 * time.Minute)
	if notBefore.IsZero() {
		notBeforeUTC = time.Time{}
	}
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, hotmailGraphAPI+"/me/messages?$top=15&$select=subject,bodyPreview,body,receivedDateTime,from&$orderby=receivedDateTime%20desc", nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("Authorization", "Bearer "+accessToken)
		resp, err := client.Do(req)
		if err == nil {
			raw, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode == 401 {
				if tok, rerr := RefreshHotmailToken(ctx, client, acc); rerr == nil {
					accessToken = tok
				}
			} else if resp.StatusCode == 200 {
				var list struct {
					Value []struct {
						Subject          string `json:"subject"`
						BodyPreview      string `json:"bodyPreview"`
						ReceivedDateTime string `json:"receivedDateTime"`
						From             struct {
							EmailAddress struct {
								Address string `json:"address"`
							} `json:"emailAddress"`
						} `json:"from"`
						Body        struct {
							Content string `json:"content"`
						} `json:"body"`
					} `json:"value"`
				}
				if json.Unmarshal(raw, &list) == nil {
					for _, msg := range list.Value {
						if !notBeforeUTC.IsZero() && msg.ReceivedDateTime != "" {
							received, perr := time.Parse(time.RFC3339, msg.ReceivedDateTime)
							if perr == nil && received.Before(notBeforeUTC) {
								continue
							}
						}
						combined := msg.Subject + " " + msg.BodyPreview + " " + cleanMailBody(msg.Body.Content)
						codes := extractVerificationCodes(combined)
						if len(codes) == 0 {
							continue
						}
						sender := strings.ToLower(msg.From.EmailAddress.Address)
						if sender == "" || strings.Contains(sender, "minimax") || strings.Contains(sender, "hailuo") {
							return codes[0], nil
						}
					}
				}
			}
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(4 * time.Second):
		}
	}
	return "", fmt.Errorf("Hotmail Graph 轮询超时，未收到 %s 的验证码", acc.Email)
}

type PreflightProgress struct {
	Total     int `json:"total"`
	Checked   int `json:"checked"`
	Available int `json:"available"`
	Skipped   int `json:"skipped"`
}

func PreflightHotmailPool(ctx context.Context, path string, workers int, dialer *ProxyDialer, onProgress func(PreflightProgress)) (HotmailPoolStats, error) {
	pool, err := LoadHotmailPool(path)
	if err != nil {
		return HotmailPoolStats{}, err
	}
	if workers <= 0 {
		workers = 8
	}
	var client *http.Client
	if dialer != nil && dialer.Enabled() {
		client = dialer.HTTPClient(20 * time.Second)
	} else {
		client = &http.Client{Timeout: 20 * time.Second}
	}

	var indices []int
	for i, a := range pool.Accounts {
		if a.Status == "" || a.Status == "available" || a.Status == "reserved" {
			indices = append(indices, i)
		}
	}

	var mu sync.Mutex
	checked := 0
	avail := 0
	skipped := 0
	jobs := make(chan int, len(indices))
	for _, idx := range indices {
		jobs <- idx
	}
	close(jobs)

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				if ctx.Err() != nil {
					return
				}
				mu.Lock()
				acc := pool.Accounts[idx]
				mu.Unlock()

				_, err := RefreshHotmailToken(ctx, client, &acc)
				now := time.Now().UTC().Format(time.RFC3339)

				mu.Lock()
				if err != nil {
					pool.Accounts[idx].Status = "skipped"
					pool.Accounts[idx].LastError = err.Error()
					skipped++
				} else {
					pool.Accounts[idx].Status = "available"
					pool.Accounts[idx].RefreshToken = acc.RefreshToken
					pool.Accounts[idx].LastError = ""
					avail++
				}
				pool.Accounts[idx].UpdatedAt = now
				checked++
				if onProgress != nil {
					onProgress(PreflightProgress{
						Total:     len(indices),
						Checked:   checked,
						Available: avail,
						Skipped:   skipped,
					})
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if err := SaveHotmailPool(path, pool); err != nil {
		return HotmailPoolStats{}, err
	}
	return pool.Stats(), nil
}
