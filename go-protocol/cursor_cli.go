package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// cursorAccountResult 是单个 Cursor 账号的结果，落盘给前端和后续签到用。
type cursorAccountResult struct {
	Email     string    `json:"email"`
	PanelUser string    `json:"panelUser"`
	SignInID  string    `json:"signInId"`
	Provider  string    `json:"provider"`
	OK        bool      `json:"ok"`
	Error     string    `json:"error,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

// cursorTestDir 返回 Cursor 流程的独立工作目录，避免污染主批量产物。
func cursorTestDir(cfg Config) string {
	return filepath.Join(cfg.OutputDir, "cursor-test")
}

// prepareCursorTestPool 把主号池复制一份到测试目录，测试只动副本。
// 现在 Cursor 与 MiniMax 共用主号池、按平台分别记账，这个副本只作为老流程的兜底。
func prepareCursorTestPool(cfg Config) (string, error) {
	dir := cursorTestDir(cfg)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	pool := filepath.Join(dir, "pool.json")
	if _, err := os.Stat(pool); err == nil {
		return pool, nil
	}
	raw, err := os.ReadFile(cfg.HotmailPool)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(pool, raw, 0o600); err != nil {
		return "", err
	}
	return pool, nil
}

// saveCursorResult 把一次结果写进结果目录。
func saveCursorResult(cfg Config, res cursorAccountResult) {
	dir := filepath.Join(cursorTestDir(cfg), "results")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	name := fmt.Sprintf("%d-%s.json", res.CreatedAt.Unix(), res.Email)
	data, _ := json.MarshalIndent(res, "", "  ")
	_ = os.WriteFile(filepath.Join(dir, name), data, 0o600)
}

// runCLITestCursor 用号池里的一个 hotmail 账号跑一遍 Cursor 注册授权。
// cursorDirectProxy 为 true 时 Chrome 不走本地代理桥，直接用系统代理。
var cursorDirectProxy = os.Getenv("MINIMAX_CURSOR_DIRECT") == "1"

func runCLITestCursor(panelBase, panelKey, email string) {
	cfg := loadCLIConfig()
	cfg.applyDefaults()
	if panelBase == "" {
		panelBase = cfg.Panel2Base
	}
	if panelBase == "" {
		panelBase = cfg.PanelBase
	}
	if panelKey == "" {
		panelKey = cfg.Panel2Key
	}
	if panelKey == "" {
		panelKey = cfg.PanelKey
	}
	fmt.Printf("[cursor] 面板 %s\n", panelBase)

	var dialer *ProxyDialer
	if cfg.ProxyURL != "" {
		d, err := NewProxyDialer(cfg.ProxyURL)
		if err != nil {
			fmt.Printf("[cursor] 代理配置错误: %v\n", err)
			return
		}
		dialer = d
	}
	bridgeAddr := ""
	if dialer != nil && dialer.Enabled() && !cursorDirectProxy {
		bridge, addr, err := StartLocalHTTPBridgeWithDialer(dialer)
		if err != nil {
			fmt.Printf("[cursor] 启动本地代理桥失败: %v\n", err)
			return
		}
		defer bridge.Close()
		bridgeAddr = addr
	}

	poolPath := cfg.HotmailPool
	store := NewHotmailPoolStore(poolPath)
	var acc *HotmailAccount
	var idx int
	if email != "" {
		pool, perr := LoadHotmailPool(poolPath)
		if perr != nil {
			fmt.Printf("[cursor] 读取号池失败: %v\n", perr)
			return
		}
		for i := range pool.Accounts {
			if strings.EqualFold(pool.Accounts[i].Email, email) {
				acc = &pool.Accounts[i]
				idx = i
				break
			}
		}
		if acc == nil {
			fmt.Printf("[cursor] 号池里没有 %s\n", email)
			return
		}
	} else {
		var cerr error
		acc, idx, cerr = store.ClaimFor(PlatformCursor)
		if cerr != nil {
			fmt.Printf("[cursor] 号池取号失败: %v\n", cerr)
			return
		}
	}
	fmt.Printf("[cursor] 使用邮箱 %s\n", acc.Email)

	ctx := context.Background()
	panel, err := NewPanelClient(panelBase, panelKey, dialer, cfg.ProxyDirectHosts)
	if err != nil {
		fmt.Printf("[cursor] 面板客户端错误: %v\n", err)
		store.FinalizeFor(PlatformCursor, idx, acc.Email, "available", err.Error(), "")
		return
	}
	if err := panel.Login(ctx); err != nil {
		fmt.Printf("[cursor] 面板登录失败: %v\n", err)
		store.FinalizeFor(PlatformCursor, idx, acc.Email, "available", err.Error(), "")
		return
	}

	mailClient := &http.Client{Timeout: 30 * time.Second}
	fetchCode := func(ctx context.Context, notBefore time.Time) (string, error) {
		return PollHotmailCodeFor(ctx, mailClient, acc, notBefore, 3*time.Minute, "cursor")
	}
	fetchLink := func(ctx context.Context, notBefore time.Time) (string, error) {
		return PollHotmailLink(ctx, mailClient, acc, notBefore, 3*time.Minute, "cursor-confirm")
	}
	flow := &CursorFlow{
		Panel:      panel,
		ChromePath: cfg.ChromePath,
		ProxyBidge: bridgeAddr,
		ProfileDir: filepath.Join(cursorTestDir(cfg), "chrome-profiles"),
		FetchLink:  fetchLink,
	}
	var res *CursorResult
	attempts := 3
	for attempt := 1; attempt <= attempts; attempt++ {
		if attempt > 1 {
			fmt.Printf("[cursor] 第 %d 次尝试：换出口 IP 重试\n", attempt)
			if dialer != nil {
				dialer.RotateSession()
			}
			time.Sleep(5 * time.Second)
		}
		var aerr error
		res, aerr = flow.Execute(ctx, acc.Email, fetchCode, func(step string) {
			fmt.Printf("[cursor] %s\n", step)
		})
		err = aerr
		if err == nil {
			break
		}
		fmt.Printf("[cursor] 第 %d 次尝试失败: %v\n", attempt, err)
	}
	out := cursorAccountResult{Email: acc.Email, CreatedAt: time.Now(), Provider: cursorProviderID}
	if err != nil {
		out.Error = err.Error()
		fmt.Printf("[cursor] 失败: %v\n", err)
		store.FinalizeFor(PlatformCursor, idx, acc.Email, "available", err.Error(), "")
	} else {
		out.OK = true
		out.PanelUser = res.User
		out.SignInID = res.SignInID
		fmt.Printf("[cursor] 成功：邮箱 %s 面板账号 %s\n", res.Email, res.User)
		store.FinalizeFor(PlatformCursor, idx, acc.Email, "used", "", acc.RefreshToken)
		store.MarkPlatformUsed(PlatformCursor, idx, acc.Email, res.User, res.SignInID)
	}
	saveCursorResult(cfg, out)
}

// runCLICursorBatch 从号池连续取号跑 Cursor 授权，成功 count 个后收工。
func runCLICursorBatch(panelBase, panelKey string, count int) {
	cfg := loadCLIConfig()
	cfg.applyDefaults()
	if panelBase == "" {
		panelBase = cfg.Panel2Base
	}
	if panelBase == "" {
		panelBase = cfg.PanelBase
	}
	if panelKey == "" {
		panelKey = cfg.Panel2Key
	}
	if panelKey == "" {
		panelKey = cfg.PanelKey
	}
	if count <= 0 {
		count = 1
	}
	fmt.Printf("[cursor] 批量开始：目标 %d 个，面板 %s\n", count, panelBase)

	var dialer *ProxyDialer
	if cfg.ProxyURL != "" {
		if d, err := NewProxyDialer(cfg.ProxyURL); err == nil {
			dialer = d
		}
	}
	poolPath := cfg.HotmailPool
	store := NewHotmailPoolStore(poolPath)
	ctx := context.Background()
	panel, err := NewPanelClient(panelBase, panelKey, dialer, cfg.ProxyDirectHosts)
	if err == nil {
		err = panel.Login(ctx)
	}
	if err != nil {
		fmt.Printf("[cursor] 面板登录失败: %v\n", err)
		return
	}
	mailClient := &http.Client{Timeout: 30 * time.Second}
	ok, fail := 0, 0
	for ok < count {
		acc, idx, cerr := store.ClaimFor(PlatformCursor)
		if cerr != nil {
			fmt.Printf("[cursor] 号池取号失败: %v\n", cerr)
			break
		}
		flow := &CursorFlow{
			Panel:      panel,
			ChromePath: cfg.ChromePath,
			ProxyBidge: "",
			ProfileDir: filepath.Join(cursorTestDir(cfg), "chrome-profiles"),
			FetchLink: func(ctx context.Context, notBefore time.Time) (string, error) {
				return PollHotmailLink(ctx, mailClient, acc, notBefore, 3*time.Minute, "cursor-confirm")
			},
		}
		res, rerr := flow.Execute(ctx, acc.Email, func(ctx context.Context, notBefore time.Time) (string, error) {
			return PollHotmailCodeFor(ctx, mailClient, acc, notBefore, 3*time.Minute, "cursor")
		}, func(step string) {
			fmt.Printf("[cursor][%s] %s\n", acc.Email, step)
		})
		out := cursorAccountResult{Email: acc.Email, CreatedAt: time.Now(), Provider: cursorProviderID}
		if rerr != nil {
			fail++
			out.Error = rerr.Error()
			store.FinalizeFor(PlatformCursor, idx, acc.Email, "available", rerr.Error(), "")
			fmt.Printf("[cursor] 失败: %v\n", rerr)
		} else {
			ok++
			out.OK = true
			out.PanelUser = res.User
			out.SignInID = res.SignInID
			store.FinalizeFor(PlatformCursor, idx, acc.Email, "used", "", acc.RefreshToken)
			store.MarkPlatformUsed(PlatformCursor, idx, acc.Email, res.User, res.SignInID)
			fmt.Printf("[cursor] 成功 %d/%d：%s -> 面板账号 %s\n", ok, count, res.Email, res.User)
		}
		saveCursorResult(cfg, out)
	}
	fmt.Printf("[cursor] 批量结束：成功 %d，失败尝试 %d\n", ok, fail)
}
