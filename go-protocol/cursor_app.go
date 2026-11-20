package main

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"sync"
	"time"
)

// cursorRunState 记录 Cursor 绑定任务的状态。
var cursorRunState struct {
	mu            sync.Mutex
	running       bool
	cancel        context.CancelFunc
	target        int
	succeeded     int
	failed        int
	currentEmail  string
	updatedAt     string
	lastPanelUser string
}

// CursorProgress 是 Cursor 绑定任务的进度快照。
type CursorProgress struct {
	Target      int    `json:"target"`
	Succeeded   int    `json:"succeeded"`
	Failed      int    `json:"failed"`
	CurrentMail string `json:"currentEmail"`
	Running     bool   `json:"running"`
	UpdatedAt   string `json:"updatedAt"`
	LastPanel   string `json:"lastPanelUser"`
}

// GetCursorProgress 返回 Cursor 绑定任务进度。
func (a *App) GetCursorProgress() CursorProgress {
	cursorRunState.mu.Lock()
	defer cursorRunState.mu.Unlock()
	return CursorProgress{
		Target:      cursorRunState.target,
		Succeeded:   cursorRunState.succeeded,
		Failed:      cursorRunState.failed,
		CurrentMail: cursorRunState.currentEmail,
		Running:     cursorRunState.running,
		UpdatedAt:   cursorRunState.updatedAt,
		LastPanel:   cursorRunState.lastPanelUser,
	}
}

// PlatformStatus 汇总当前任务平台与运行状态（前端用来显示"现在在跑什么"）。
type PlatformStatus struct {
	Mode           string         `json:"mode"`
	ModeLabel      string         `json:"modeLabel"`
	MiniMax        bool           `json:"miniMax"`
	Cursor         bool           `json:"cursor"`
	MiniMaxRunning bool           `json:"miniMaxRunning"`
	Supervised     bool           `json:"supervised"`
	CursorRunning  bool           `json:"cursorRunning"`
	CursorProgress CursorProgress `json:"cursorProgress"`
}

// GetPlatformStatus 返回当前运行平台与各自运行状态。
func (a *App) GetPlatformStatus(cfg Config) PlatformStatus {
	cfg.applyDefaults()
	a.supM.Lock()
	sup := a.supRunning
	a.supM.Unlock()
	cp := a.GetCursorProgress()
	return PlatformStatus{
		Mode:           normalizePlatformMode(cfg.PlatformMode),
		ModeLabel:      cfg.PlatformModeLabel(),
		MiniMax:        cfg.RunMiniMax(),
		Cursor:         cfg.RunCursor(),
		MiniMaxRunning: a.runner.IsRunning(),
		Supervised:     sup,
		CursorRunning:  cp.Running,
		CursorProgress: cp,
	}
}

// StopCursorBinding 停止 Cursor 绑定任务。
func (a *App) StopCursorBinding() {
	cursorRunState.mu.Lock()
	defer cursorRunState.mu.Unlock()
	if cursorRunState.cancel != nil {
		cursorRunState.cancel()
		cursorRunState.cancel = nil
	}
}

// StartCursorBinding 用号池里的 Hotmail 账号跑 Cursor 授权绑定，成功 count 个后收工。
func (a *App) StartCursorBinding(cfg Config, count int) error {
	cfg.applyDefaults()
	if err := SaveConfig(cfg); err != nil {
		return err
	}
	if count <= 0 {
		count = 1
	}
	cursorRunState.mu.Lock()
	if cursorRunState.running {
		cursorRunState.mu.Unlock()
		return fmt.Errorf("Cursor 绑定已在运行中")
	}
	base := a.ctx
	if base == nil {
		base = context.Background()
	}
	ctx, cancel := context.WithCancel(base)
	cursorRunState.cancel = cancel
	cursorRunState.running = true
	cursorRunState.target = count
	cursorRunState.succeeded = 0
	cursorRunState.failed = 0
	cursorRunState.currentEmail = ""
	cursorRunState.updatedAt = time.Now().Format("15:04:05")
	cursorRunState.mu.Unlock()
	go a.runCursorBinding(ctx, cfg, count)
	return nil
}

// runCursorBinding 循环跑 Cursor 绑定，直到成功 count 个、号池取空或被停止。
func (a *App) runCursorBinding(ctx context.Context, cfg Config, count int) {
	ok, fail := 0, 0
	defer func() {
		cursorRunState.mu.Lock()
		cursorRunState.running = false
		cursorRunState.mu.Unlock()
		a.emitBatchLog("info", fmt.Sprintf("[cursor] 已停止：成功 %d/%d，失败尝试 %d", ok, count, fail))
	}()
	panelBase := cfg.Panel2Base
	if panelBase == "" {
		panelBase = cfg.PanelBase
	}
	panelKey := cfg.Panel2Key
	if panelKey == "" {
		panelKey = cfg.PanelKey
	}
	var dialer *ProxyDialer
	if cfg.ProxyURL != "" {
		if d, err := NewProxyDialer(cfg.ProxyURL); err == nil {
			dialer = d
		}
	}
	// Cursor 浏览器单独配一个出口：Cursor 对数据中心 IP 很敏感，
	// 数据中心 IP 会被 cursor.com 直接 reset，需要住宅代理。
	bridgeAddr := ""
	browserProxy := cfg.CursorProxy
	if browserProxy == "" {
		browserProxy = cfg.ProxyURL
	}
	if browserProxy != "" {
		bd, berr := NewProxyDialer(browserProxy)
		if berr == nil && bd.Enabled() {
			bridge, addr, err := StartLocalHTTPBridgeWithDialer(bd)
			if err == nil {
				defer bridge.Close()
				bridgeAddr = addr
			}
		}
	}
	if bridgeAddr == "" {
		a.emitBatchLog("info", "[cursor] 浏览器直连（未配置 Cursor 代理）")
	} else {
		a.emitBatchLog("info", "[cursor] 浏览器走独立代理出口 "+bridgeAddr)
	}
	poolPath := cfg.HotmailPool
	store := NewHotmailPoolStore(poolPath)
	panel, err := NewPanelClient(panelBase, panelKey, dialer, cfg.ProxyDirectHosts)
	if err == nil {
		err = panel.Login(ctx)
	}
	if err != nil {
		a.emitBatchLog("error", "[cursor] 面板登录失败: "+err.Error())
		return
	}
	mailClient := &http.Client{Timeout: 30 * time.Second}
	for ok < count {
		if ctx.Err() != nil {
			return
		}
		acc, idx, cerr := store.ClaimFor(PlatformCursor)
		if cerr != nil {
			a.emitBatchLog("error", "[cursor] 号池取号失败: "+cerr.Error())
			return
		}
		cursorRunState.mu.Lock()
		cursorRunState.currentEmail = acc.Email
		cursorRunState.updatedAt = time.Now().Format("15:04:05")
		cursorRunState.mu.Unlock()
		flow := &CursorFlow{
			Panel:      panel,
			ChromePath: cfg.ChromePath,
			ProxyBidge: bridgeAddr,
			ProfileDir: filepath.Join(cursorTestDir(cfg), "chrome-profiles"),
			FetchLink: func(ctx context.Context, notBefore time.Time) (string, error) {
				return PollHotmailLink(ctx, mailClient, acc, notBefore, 3*time.Minute, "cursor-confirm")
			},
		}
		res, rerr := flow.Execute(ctx, acc.Email, func(ctx context.Context, notBefore time.Time) (string, error) {
			return PollHotmailCodeFor(ctx, mailClient, acc, notBefore, 3*time.Minute, "cursor")
		}, func(step string) {
			a.emitBatchLog("info", "["+acc.Email+"] "+step)
		})
		if rerr != nil {
			fail++
			cursorRunState.mu.Lock()
			cursorRunState.failed = fail
			cursorRunState.updatedAt = time.Now().Format("15:04:05")
			cursorRunState.mu.Unlock()
			store.FinalizeFor(PlatformCursor, idx, acc.Email, "available", rerr.Error(), "")
			a.emitBatchLog("error", "["+acc.Email+"] "+rerr.Error())
		} else {
			ok++
			cursorRunState.mu.Lock()
			cursorRunState.succeeded = ok
			cursorRunState.lastPanelUser = res.User
			cursorRunState.updatedAt = time.Now().Format("15:04:05")
			cursorRunState.mu.Unlock()
			store.FinalizeFor(PlatformCursor, idx, acc.Email, "used", "", acc.RefreshToken)
			store.MarkPlatformUsed(PlatformCursor, idx, acc.Email, res.User, res.SignInID)
			a.emitBatchLog("info", fmt.Sprintf("[cursor] 成功 %d/%d：%s -> 面板账号 %s", ok, count, acc.Email, res.User))
		}
		emitEvent("batch:progress", ProgressLite{
			Target: count, Succeeded: ok, FailedAttempts: fail,
			CurrentStage: "Cursor 授权绑定", CurrentEmail: acc.Email,
			Running: true, UpdatedAt: time.Now().Format("15:04:05"),
		})
	}
}
