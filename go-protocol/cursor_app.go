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
	mu      sync.Mutex
	running bool
	cancel  context.CancelFunc
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
	// 浏览器走系统代理直连（本地代理桥对 cursor.com 的 CONNECT 常被上游拒绝），
	// 面板与 Graph 邮箱请求仍走配置的上游代理。
	bridgeAddr := ""
	a.emitBatchLog("info", "[cursor] 浏览器走系统代理，面板与邮箱请求走配置代理")
	poolPath, err := prepareCursorTestPool(cfg)
	if err != nil {
		a.emitBatchLog("error", "[cursor] 准备号池失败: "+err.Error())
		return
	}
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
		acc, idx, cerr := store.Claim()
		if cerr != nil {
			a.emitBatchLog("error", "[cursor] 号池取号失败: "+cerr.Error())
			return
		}
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
			store.Finalize(idx, acc.Email, "available", rerr.Error(), "")
			a.emitBatchLog("error", "["+acc.Email+"] "+rerr.Error())
		} else {
			ok++
			store.Finalize(idx, acc.Email, "used", "", acc.RefreshToken)
			a.emitBatchLog("info", fmt.Sprintf("[cursor] 成功 %d/%d：%s -> 面板账号 %s", ok, count, acc.Email, res.User))
		}
		emitEvent("batch:progress", ProgressLite{
			Target: count, Succeeded: ok, FailedAttempts: fail,
			CurrentStage: "Cursor 授权绑定", CurrentEmail: acc.Email,
			Running: true, UpdatedAt: time.Now().Format("15:04:05"),
		})
	}
}
