package main

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
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
			d.SetOptionsEx(splitRegions(cfg.ProxyRegionRotate), cfg.ProxySessTime,
				parseProxyStyle(cfg.ProxyStyle), cfg.ProxyState, cfg.StickyEnabled(), cfg.ProxyRotateSession)
			dialer = d
		}
	}
	// 节点池模式：订阅里存活的节点轮流当出口，每个账号换一次 IP。
	var poolSpecs []proxyNodeSpec
	if cfg.ProxyPoolUse {
		pool, perr := LoadProxyPool(proxyPoolPath(cfg))
		if perr != nil {
			a.emitBatchLog("error", "[cursor] 读取节点池失败（"+perr.Error()+"），回退固定代理")
		} else {
			poolSpecs = nodesToDialerSpecs(pool.Nodes)
			if len(poolSpecs) == 0 {
				a.emitBatchLog("error", "[cursor] 节点池里没有存活节点，改用固定代理")
				poolSpecs = nil
			} else {
				a.emitBatchLog("info", fmt.Sprintf("[cursor] 节点池已启用：%d 个存活节点，每个账号轮换出口 IP", len(poolSpecs)))
			}
		}
	}
	if dialer != nil && len(poolSpecs) > 0 {
		dialer.SetPoolNodes(poolSpecs, 0)
	}
	// Cursor 浏览器单独配一个出口：Cursor 对数据中心 IP 很敏感，
	// 数据中心 IP 会被 cursor.com 直接 reset，需要住宅代理。
	bridgeAddr := ""
	browserProxy := cfg.CursorProxy
	if browserProxy == "" {
		browserProxy = cfg.ProxyURL
	}
	var browserDialer *ProxyDialer
	if browserProxy != "" {
		bd, berr := NewProxyDialer(browserProxy)
		if berr == nil {
			bd.SetOptionsEx(splitRegions(cfg.ProxyRegionRotate), cfg.ProxySessTime,
				parseProxyStyle(cfg.ProxyStyle), cfg.ProxyState, cfg.StickyEnabled(), cfg.ProxyRotateSession)
			if len(poolSpecs) > 0 {
				bd.SetPoolNodes(poolSpecs, 0)
			}
		}
		if berr == nil && bd.Enabled() {
			bridge, addr, err := StartLocalHTTPBridgeWithDialer(bd)
			if err == nil {
				defer bridge.Close()
				bridgeAddr = addr
			}
			browserDialer = bd
		}
	}
	if bridgeAddr == "" {
		a.emitBatchLog("info", "[cursor] 浏览器直连（未配置 Cursor 代理）")
	} else {
		a.emitBatchLog("info", "[cursor] 浏览器走独立代理出口 "+bridgeAddr)
	}
	poolPath := cfg.HotmailPool
	store := NewHotmailPoolStore(poolPath)
	// 面板可以配两个，账号在两个面板之间轮流绑定：user_code / 授权链接互不干扰。
	panels := make([]*PanelClient, 0, 2)
	addPanel := func(base, key, label string) {
		base = strings.TrimSpace(base)
		key = strings.TrimSpace(key)
		if base == "" || key == "" {
			return
		}
		for _, p := range panels {
			if p.baseURL == base {
				return
			}
		}
		pc, err := NewPanelClient(base, key, dialer, cfg.ProxyDirectHosts)
		if err != nil {
			a.emitBatchLog("error", "[cursor] 面板 "+label+" 初始化失败: "+err.Error())
			return
		}
		if err := pc.Login(ctx); err != nil {
			a.emitBatchLog("error", "[cursor] 面板 "+label+" 登录失败: "+err.Error())
			return
		}
		panels = append(panels, pc)
		a.emitBatchLog("info", "[cursor] 面板 "+label+" 已就绪: "+base)
	}
	addPanel(cfg.PanelBase, cfg.PanelKey, "1")
	addPanel(cfg.Panel2Base, cfg.Panel2Key, "2")
	if len(panels) == 0 {
		a.emitBatchLog("error", "[cursor] 没有可用面板")
		return
	}
	mailClient := &http.Client{Timeout: 30 * time.Second}
	if n := store.ReleaseReserved(PlatformCursor); n > 0 {
		a.emitBatchLog("info", fmt.Sprintf("[cursor] 已释放 %d 个上次残留的占用邮箱", n))
	}
	turn := 0
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
		panel := panels[turn%len(panels)]
		turn++
		a.emitBatchLog("info", "["+acc.Email+"] 分配到面板: "+panel.baseURL)
		// 每个账号换一个出口 IP（节点池模式下换节点，固定代理模式下换会话）。
		if dialer != nil && len(poolSpecs) > 0 {
			dialer.RotateSession()
		}
		if browserDialer != nil && len(poolSpecs) > 0 {
			browserDialer.RotateSession()
		}
		if name := browserDialer.CurrentNodeName(); name != "" {
			a.emitBatchLog("info", "["+acc.Email+"] 本次出口节点: "+name)
		} else if ip := exitIPOf(browserDialer); ip != "" {
			a.emitBatchLog("info", "["+acc.Email+"] 本次出口 IP: "+ip)
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
		// 单个账号最多试 2 次：第一次失败多半是出口 IP 不行，换节点/换会话再来一次。
		var res *CursorResult
		var rerr error
		for attempt := 1; attempt <= 2; attempt++ {
			res, rerr = flow.Execute(ctx, acc.Email, func(ctx context.Context, notBefore time.Time) (string, error) {
				return PollHotmailCodeFor(ctx, mailClient, acc, notBefore, 3*time.Minute, "cursor")
			}, func(step string) {
				a.emitBatchLog("info", "["+acc.Email+"] "+step)
				// 每一步都刷新一下状态，界面上才看得出还在动
				cursorRunState.mu.Lock()
				cursorRunState.currentEmail = acc.Email
				cursorRunState.updatedAt = time.Now().Format("15:04:05")
				cursorRunState.mu.Unlock()
			})
			if rerr == nil {
				break
			}
			if attempt == 2 {
				break
			}
			a.emitBatchLog("info", "["+acc.Email+"] 第 1 次失败（"+rerr.Error()+"），换出口 IP 重试")
			if dialer != nil {
				dialer.RotateSession()
			}
			if browserDialer != nil {
				browserDialer.RotateSession()
			}
			time.Sleep(3 * time.Second)
		}
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
