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
	Status       string `json:"status"` // available | reserved | used | skipped（整体状态，等价于 Platforms[默认平台]）
	FailCount    int    `json:"failCount,omitempty"`
	LastError    string `json:"lastError,omitempty"`
	UpdatedAt    string `json:"updatedAt,omitempty"`
	// Platforms 记录这个邮箱在各个平台上的使用状态，键是平台 ID（如 minimax / cursor）。
	// 同一个邮箱可以在 MiniMax 上注册过，也照样能拿去绑 Cursor，互不影响。
	Platforms map[string]*PlatformState `json:"platforms,omitempty"`
}

// PlatformState 是某个邮箱在单个平台上的状态。
type PlatformState struct {
	Status       string `json:"status"` // available | reserved | used | skipped
	PanelUser    string `json:"panelUser,omitempty"`
	SignInID     string `json:"signInId,omitempty"`
	LastError    string `json:"lastError,omitempty"`
	UpdatedAt    string `json:"updatedAt,omitempty"`
	FailCount    int    `json:"failCount,omitempty"`
	RegisteredAt string `json:"registeredAt,omitempty"`
}

// 平台 ID 常量。
const (
	PlatformMiniMax = "minimax"
	PlatformCursor  = "cursor"
)

// PlatformStatus 取账号在指定平台上的状态，没有记录时按整体 Status 兜底。
func (a *HotmailAccount) PlatformStatus(platform string) string {
	if a != nil && a.Platforms != nil {
		if st, ok := a.Platforms[platform]; ok && st != nil && st.Status != "" {
			return st.Status
		}
	}
	if a == nil {
		return "available"
	}
	// skipped = 邮箱令牌本身失效，对所有平台都不可用。
	if strings.TrimSpace(a.Status) == "skipped" {
		return "skipped"
	}
	// 其余平台还没有记录，说明这个邮箱在那个平台上还没被用过。
	return "available"
}

// setPlatformStatus 写入某个平台的状态，同时同步整体状态。
func (a *HotmailAccount) setPlatformStatus(platform, status string) {
	if a.Platforms == nil {
		a.Platforms = map[string]*PlatformState{}
	}
	st, ok := a.Platforms[platform]
	if !ok || st == nil {
		st = &PlatformState{}
		a.Platforms[platform] = st
	}
	st.Status = status
	st.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if platform == PlatformMiniMax || len(a.Platforms) == 1 {
		// MiniMax 是主流程，整体状态跟着它走，保证老界面/老统计不变。
		a.Status = status
	}
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
	// ByPlatform 是各平台分别的统计，方便看清同一个邮箱在 MiniMax / Cursor 上的不同状态。
	ByPlatform map[string]HotmailPoolStats `json:"byPlatform,omitempty"`
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
	migrateHotmailPlatforms(&p)
	return &p, nil
}

// migrateHotmailPlatforms 给老号池补上平台维度：
// 老的 status=used 表示这个邮箱在 MiniMax 上注册过，Cursor 那边仍然可用。
func migrateHotmailPlatforms(p *HotmailPool) {
	if p == nil {
		return
	}
	for i := range p.Accounts {
		acc := &p.Accounts[i]
		if acc.Platforms == nil {
			acc.Platforms = map[string]*PlatformState{}
		}
		status := strings.TrimSpace(acc.Status)
		if status == "" {
			status = "available"
		}
		st, ok := acc.Platforms[PlatformMiniMax]
		if !ok || st == nil {
			acc.Platforms[PlatformMiniMax] = &PlatformState{
				Status:    status,
				FailCount: acc.FailCount,
				LastError: acc.LastError,
				UpdatedAt: acc.UpdatedAt,
			}
		}
	}
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

// poolFileLocks 保证同一个号池文件在同进程内只有一个写入者，
// 不同流程（MiniMax / Cursor）共用号池时不会互相覆盖。
var poolFileLocks sync.Map

func poolFileLock(path string) *sync.Mutex {
	v, _ := poolFileLocks.LoadOrStore(path, &sync.Mutex{})
	return v.(*sync.Mutex)
}

// lockPoolFile 拿跨进程文件锁，防止两个程序同时改同一个号池文件。
func lockPoolFile(path string) (func(), error) {
	lockPath := path + ".lock"
	deadline := time.Now().Add(30 * time.Second)
	for {
		f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			_ = f.Close()
			return func() { _ = os.Remove(lockPath) }, nil
		}
		if !os.IsExist(err) {
			return nil, err
		}
		// 锁文件超过 60 秒认为是残留，强制清掉。
		if info, statErr := os.Stat(lockPath); statErr == nil && time.Since(info.ModTime()) > 60*time.Second {
			_ = os.Remove(lockPath)
			continue
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("等待号池文件锁超时: %s", lockPath)
		}
		time.Sleep(50 * time.Millisecond)
	}
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
	return s.ClaimFor(PlatformMiniMax)
}

// ClaimFor 按平台领取一个可用邮箱。同一邮箱在不同平台上互不影响。
func (s *HotmailPoolStore) ClaimFor(platform string) (*HotmailAccount, int, error) {
	if platform == "" {
		platform = PlatformMiniMax
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	fl := poolFileLock(s.path)
	fl.Lock()
	defer fl.Unlock()
	unlock, err := lockPoolFile(s.path)
	if err != nil {
		return nil, -1, err
	}
	defer unlock()
	pool, err := LoadHotmailPool(s.path)
	if err != nil {
		return nil, -1, err
	}
	best := -1
	var bestAt time.Time
	for i := range pool.Accounts {
		status := strings.TrimSpace(pool.Accounts[i].PlatformStatus(platform))
		if status != "" && status != "available" {
			continue
		}
		at := pool.Accounts[i].platformUpdatedAt(platform)
		if best < 0 || at.Before(bestAt) {
			best = i
			bestAt = at
		}
	}
	if best < 0 {
		return nil, -1, ErrHotmailPoolExhausted
	}
	pool.Accounts[best].setPlatformStatus(platform, "reserved")
	acc := pool.Accounts[best]
	if err := SaveHotmailPool(s.path, pool); err != nil {
		return nil, -1, err
	}
	return &acc, best, nil
}

// platformUpdatedAt 返回该账号在这个平台上的最后更新时间，用于先进先出。
func (a *HotmailAccount) platformUpdatedAt(platform string) time.Time {
	if a != nil && a.Platforms != nil {
		if st, ok := a.Platforms[platform]; ok && st != nil && st.UpdatedAt != "" {
			if t, err := time.Parse(time.RFC3339, st.UpdatedAt); err == nil {
				return t
			}
		}
	}
	// 这个平台还没用过 -> 视为最早，优先被领取（最大化跨平台复用）。
	return time.Time{}
}

func (s *HotmailPoolStore) Finalize(index int, email, status, lastErr, refreshToken string) {
	s.FinalizeFor(PlatformMiniMax, index, email, status, lastErr, refreshToken)
}

// FinalizeFor 按平台回写账号状态。不同平台的失败/成功互不影响。
func (s *HotmailPoolStore) FinalizeFor(platform string, index int, email, status, lastErr, refreshToken string) {
	if platform == "" {
		platform = PlatformMiniMax
	}
	if index < 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	fl := poolFileLock(s.path)
	fl.Lock()
	defer fl.Unlock()
	unlock, err := lockPoolFile(s.path)
	if err != nil {
		return
	}
	defer unlock()
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
		pool.Accounts[index].setPlatformFailCount(platform, pool.Accounts[index].platformFailCount(platform)+1)
		// 可恢复失败（IP 风控 1200058、人机 1200063、验证码过期 1200059、网络抖动等）：
		// 保持 available，UpdatedAt 刷新会自动排到队尾，下一轮重新尝试。
		// 仅当连续 10 轮全失败（确认真坏号，如令牌失效残留）才永久 skipped，避免无限空转。
		if pool.Accounts[index].platformFailCount(platform) >= 10 {
			status = "skipped"
			lastErr = "连续 10 轮失败，自动跳过: " + lastErr
		}
	}
	if status == "used" {
		pool.Accounts[index].setPlatformFailCount(platform, 0)
	}
	pool.Accounts[index].setPlatformStatus(platform, status)
	if status == "skipped" {
		// 令牌失效是邮箱级问题，所有平台都别再用它。
		for pl, st := range pool.Accounts[index].Platforms {
			if pl == platform || st == nil {
				continue
			}
			st.Status = "skipped"
			st.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		}
	}
	if platform == PlatformMiniMax {
		pool.Accounts[index].LastError = lastErr
	}
	if platform == PlatformMiniMax {
		pool.Accounts[index].FailCount = pool.Accounts[index].platformFailCount(platform)
	}
	if refreshToken != "" {
		pool.Accounts[index].RefreshToken = refreshToken
	}
	pool.Accounts[index].UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	_ = SaveHotmailPool(s.path, pool)
}

// platformFailCount 读取该平台上的连续失败次数。
func (a *HotmailAccount) platformFailCount(platform string) int {
	if a != nil && a.Platforms != nil {
		if st, ok := a.Platforms[platform]; ok && st != nil {
			return st.FailCount
		}
	}
	return 0
}

// setPlatformFailCount 写入该平台上的连续失败次数。
func (a *HotmailAccount) setPlatformFailCount(platform string, n int) {
	if a.Platforms == nil {
		a.Platforms = map[string]*PlatformState{}
	}
	st, ok := a.Platforms[platform]
	if !ok || st == nil {
		st = &PlatformState{}
		a.Platforms[platform] = st
	}
	st.FailCount = n
}

// ReleaseReserved 把某个平台上残留的占用（reserved）放回可用。
// 上一次任务崩掉时账号会一直卡在占用中，号池再也取不到它。
func (s *HotmailPoolStore) ReleaseReserved(platform string) int {
	if platform == "" {
		platform = PlatformMiniMax
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	fl := poolFileLock(s.path)
	fl.Lock()
	defer fl.Unlock()
	unlock, err := lockPoolFile(s.path)
	if err != nil {
		return 0
	}
	defer unlock()
	pool, err := LoadHotmailPool(s.path)
	if err != nil {
		return 0
	}
	released := 0
	for i := range pool.Accounts {
		if strings.TrimSpace(pool.Accounts[i].PlatformStatus(platform)) == "reserved" {
			pool.Accounts[i].setPlatformStatus(platform, "available")
			released++
		}
	}
	if released > 0 {
		_ = SaveHotmailPool(s.path, pool)
	}
	return released
}

// MarkPlatformUsed 记录某个平台上的绑定结果（面板账号、sign-in id）。
func (s *HotmailPoolStore) MarkPlatformUsed(platform string, index int, email, panelUser, signInID string) {
	if index < 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	fl := poolFileLock(s.path)
	fl.Lock()
	defer fl.Unlock()
	unlock, err := lockPoolFile(s.path)
	if err != nil {
		return
	}
	defer unlock()
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
	pool.Accounts[index].setPlatformStatus(platform, "used")
	st := pool.Accounts[index].Platforms[platform]
	if st != nil {
		st.PanelUser = panelUser
		st.SignInID = signInID
		st.RegisteredAt = time.Now().UTC().Format(time.RFC3339)
		st.FailCount = 0
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
	st.ByPlatform = p.StatsByPlatform()
	return st
}

// StatsByPlatform 统计各平台分别的可用/已用情况。
func (p *HotmailPool) StatsByPlatform() map[string]HotmailPoolStats {
	out := map[string]HotmailPoolStats{}
	if p == nil {
		return out
	}
	platforms := []string{PlatformMiniMax, PlatformCursor}
	for _, a := range p.Accounts {
		seen := map[string]bool{}
		if a.Platforms != nil {
			for k := range a.Platforms {
	seen[k] = true
			}
		}
		if len(seen) == 0 {
			seen[PlatformMiniMax] = true
		}
		for _, pl := range platforms {
			if !seen[pl] {
				continue
			}
			s := out[pl]
			s.Total++
			switch a.PlatformStatus(pl) {
			case "available":
				s.Available++
			case "reserved":
				s.Reserved++
			case "used":
				s.Used++
			case "skipped":
				s.Skipped++
			default:
				s.Available++
			}
			out[pl] = s
		}
	}
	return out
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
			// 重新导入视为补齐凭据，清掉各平台的跳过状态，但保留已用标记。
			for _, st := range pool.Accounts[idx].Platforms {
				if st != nil && st.Status == "skipped" {
					st.Status = "available"
					st.FailCount = 0
					st.LastError = ""
					st.UpdatedAt = now
				}
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
				Platforms: map[string]*PlatformState{
					PlatformMiniMax: {Status: "available", UpdatedAt: now},
				},
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
	return PollHotmailCodeFor(ctx, client, acc, notBefore, timeout, "minimax")
}

// PollHotmailCodeFor 按邮件来源过滤验证码：kind 取 minimax 或 cursor。
func PollHotmailCodeFor(ctx context.Context, client *http.Client, acc *HotmailAccount, notBefore time.Time, timeout time.Duration, kind string) (string, error) {
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
						subject := strings.ToLower(msg.Subject)
						if mailKindMatches(kind, sender, subject) {
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

// mailKindMatches 判断一封邮件是否是目标 provider 发的验证码邮件。
func mailKindMatches(kind, sender, subject string) bool {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "cursor":
		if strings.Contains(subject, "confirm a change") {
			return false
		}
		return strings.Contains(sender, "cursor") || strings.Contains(subject, "cursor") || strings.Contains(subject, "登录 cursor")
	case "cursor-confirm":
		return strings.Contains(subject, "confirm") || strings.Contains(subject, "确认")
	default:
		return sender == "" || strings.Contains(sender, "minimax") || strings.Contains(sender, "hailuo")
	}
}

// linkPattern 抓邮件正文里的 http 链接。
var linkPattern = regexp.MustCompile(`https?://[^\s"'<>)\\]+`)

// PollHotmailLink 轮询邮箱，取回最新一封目标邮件里的链接（Cursor 的确认信用）。
func PollHotmailLink(ctx context.Context, client *http.Client, acc *HotmailAccount, notBefore time.Time, timeout time.Duration, kind string) (string, error) {
	accessToken, err := RefreshHotmailToken(ctx, client, acc)
	if err != nil {
		return "", err
	}
	deadline := time.Now().Add(timeout)
	notBeforeUTC := notBefore.UTC().Add(-2 * time.Minute)
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, hotmailGraphAPI+"/me/messages?$top=10&$select=subject,bodyPreview,body,receivedDateTime,from&$orderby=receivedDateTime%20desc", nil)
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
						Body struct {
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
						sender := strings.ToLower(msg.From.EmailAddress.Address)
						subject := strings.ToLower(msg.Subject)
						if !mailKindMatches(kind, sender, subject) {
							continue
						}
						rawBody := strings.ReplaceAll(msg.Body.Content, "&amp;", "&")
						rawBody = strings.ReplaceAll(rawBody, "&#38;", "&")
						cands := linkPattern.FindAllString(msg.BodyPreview+" "+rawBody, -1)
						if os.Getenv("MINIMAX_CURSOR_DEBUG") == "1" {
							for _, lk := range cands {
								fmt.Printf("[cursor-mail] %s | %s | %s\n", msg.ReceivedDateTime, msg.Subject, lk)
							}
						}
						for _, link := range cands {
							if strings.Contains(strings.ToLower(link), "confirm-action") {
								return link, nil
							}
						}
						for _, link := range cands {
							if strings.Contains(strings.ToLower(link), "cursor") {
								return link, nil
							}
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
	return "", fmt.Errorf("Hotmail Graph 轮询超时，未收到 %s 的确认链接", acc.Email)
}
