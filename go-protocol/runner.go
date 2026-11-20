package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type AccountResult struct {
	Attempt      int    `json:"attempt"`
	Email        string `json:"email"`
	ProviderName string `json:"providerName"`
	UserCode     string `json:"userCode"`
	SignInID     string `json:"signInId"`
	OK           bool   `json:"ok"`
	Error        string `json:"error,omitempty"`
	Signin       string `json:"signin,omitempty"`
	CreatedAt    string `json:"createdAt"`
}

type BatchSummary struct {
	Target         int             `json:"target"`
	Succeeded      int             `json:"succeeded"`
	FailedAttempts int             `json:"failedAttempts"`
	CurrentStage   string          `json:"currentStage"`
	CurrentEmail   string          `json:"currentEmail"`
	Running        bool            `json:"running"`
	UpdatedAt      string          `json:"updatedAt"`
	Results        []AccountResult `json:"results"`
}

type ProtocolRunner struct {
	mu         sync.Mutex
	cancel     context.CancelFunc
	running    bool
	attemptSeq int64
	summary    BatchSummary
	onLog      func(level, msg string)
	onState    func(BatchSummary)
}

var bindingCeremonyMu sync.Mutex

func NewProtocolRunner(onLog func(level, msg string), onState func(BatchSummary)) *ProtocolRunner {
	return &ProtocolRunner{
		onLog:   onLog,
		onState: onState,
	}
}

func (r *ProtocolRunner) IsRunning() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.running
}

func (r *ProtocolRunner) Summary() BatchSummary {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.summary
}

func (r *ProtocolRunner) Stop() {
	r.mu.Lock()
	cancel := r.cancel
	r.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (r *ProtocolRunner) emitLog(level, msg string) {
	if r.onLog != nil {
		r.onLog(level, msg)
	}
}

func (r *ProtocolRunner) emitState() {
	r.mu.Lock()
	r.summary.UpdatedAt = time.Now().Format(time.RFC3339)
	s := r.summary
	r.mu.Unlock()
	if r.onState != nil {
		r.onState(s)
	}
}

func randomAlnum(n int) string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		b[i] = chars[num.Int64()]
	}
	return string(b)
}

// randomPassword 生成符合强度要求的随机密码：
// 长度 12，强制包含大写字母/小写字母/数字/特殊字符各至少 1 个，且完全随机无固定前后缀。
func randomPassword() string {
	const upper = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	const lower = "abcdefghijklmnopqrstuvwxyz"
	const digits = "0123456789"
	const special = "!@#$%^&*-_=+"
	const all = upper + lower + digits + special

	pick := func(s string) byte {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(s))))
		return s[n.Int64()]
	}

	const length = 12
	b := make([]byte, length)
	// 先保证四类字符各占 1 位
	b[0] = pick(upper)
	b[1] = pick(lower)
	b[2] = pick(digits)
	b[3] = pick(special)
	for i := 4; i < length; i++ {
		b[i] = pick(all)
	}
	// 随机打乱，避免固定位置模式
	for i := length - 1; i > 0; i-- {
		j, _ := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		b[i], b[j.Int64()] = b[j.Int64()], b[i]
	}
	return string(b)
}

func (r *ProtocolRunner) Start(cfg Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return fmt.Errorf("任务已在运行中")
	}
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	r.running = true
	r.summary = BatchSummary{
		Target:       cfg.Count,
		CurrentStage: "初始化纯 Go 协议引擎",
		Running:      true,
		Results:      []AccountResult{},
	}
	r.mu.Unlock()

	summaryPath := filepath.Join(cfg.OutputDir, "summary.json")
	if raw, err := os.ReadFile(summaryPath); err == nil {
		var prev BatchSummary
		if json.Unmarshal(raw, &prev) == nil {
			r.mu.Lock()
			r.summary.Succeeded = prev.Succeeded
			r.summary.FailedAttempts = prev.FailedAttempts
			r.summary.Results = prev.Results
			r.mu.Unlock()
		}
	}
	r.emitState()

	go r.loop(ctx, cfg, summaryPath)
	return nil
}

func (r *ProtocolRunner) saveSummary(path string) {
	r.mu.Lock()
	data, _ := json.MarshalIndent(r.summary, "", "  ")
	r.mu.Unlock()
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	_ = os.WriteFile(path, data, 0o600)
}

func (r *ProtocolRunner) loop(ctx context.Context, cfg Config, summaryPath string) {
	defer func() {
		r.mu.Lock()
		r.running = false
		r.summary.Running = false
		r.summary.CurrentStage = "已停止"
		r.mu.Unlock()
		r.saveSummary(summaryPath)
		r.emitState()
	}()

	workers := cfg.Workers
	if workers < 1 {
		workers = 1
	}
	if workers > 16 {
		workers = 16
	}

	panelDialer, err := NewProxyDialer(cfg.ProxyURL)
	if err != nil {
		r.emitLog("error", "代理配置错误: "+err.Error())
		return
	}
	panel, err := NewPanelClient(cfg.PanelBase, cfg.PanelKey, panelDialer, cfg.ProxyDirectHosts)
	if err != nil {
		r.emitLog("error", "初始化面板客户端失败: "+err.Error())
		return
	}
	if err := panel.Login(ctx); err != nil {
		r.emitLog("error", "面板登录失败: "+err.Error())
		return
	}
	panels := []*PanelClient{panel}
	if strings.TrimSpace(cfg.Panel2Base) != "" && strings.TrimSpace(cfg.Panel2Key) != "" {
		p2, err := NewPanelClient(cfg.Panel2Base, cfg.Panel2Key, panelDialer, cfg.ProxyDirectHosts)
		if err != nil {
			r.emitLog("error", "初始化第二面板失败，退回单面板: "+err.Error())
		} else {
			if err := p2.Login(ctx); err != nil {
				r.emitLog("error", "第二面板登录失败，退回单面板: "+err.Error())
			} else {
				panels = append(panels, p2)
				r.emitLog("info", "第二面板登录成功")
			}
		}
	}
	r.emitLog("info", "面板登录成功")
	r.emitLog("info", fmt.Sprintf("纯 Go 协议并行 %d 路启动（取票模式=%s，面板数=%d）", workers, cfg.TicketMode, len(panels)))

	if cfg.MailMode == "hotmail" {
		if recovered, err := RecoverStaleHotmailReservations(cfg.HotmailPool, 10*time.Minute); err != nil {
			r.emitLog("error", fmt.Sprintf("回收中断任务账号失败: %s", err.Error()))
		} else if recovered > 0 {
			r.emitLog("info", fmt.Sprintf("已回收上次中断遗留的 %d 个 Hotmail 账号", recovered))
		}
	}

	store := NewHotmailPoolStore(cfg.HotmailPool)

	var wg sync.WaitGroup
	for w := 1; w <= workers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			// 每个线程固定绑一个面板（按线程号取模），同面板内仪式串行、跨面板并行
			pick := panels[workerID%len(panels)]
			r.emitLog("info", fmt.Sprintf("[%d] worker \u7ed1\u5b9a\u9762\u677f: %s", workerID, pick.baseURL))
			r.workerLoop(ctx, cfg, workerID, pick, store, summaryPath)
		}(w)
	}
	wg.Wait()
}

func (r *ProtocolRunner) workerLoop(ctx context.Context, cfg Config, workerID int, panel *PanelClient, store *HotmailPoolStore, summaryPath string) {
	dialer, err := NewProxyDialer(cfg.ProxyURL)
	if err != nil {
		r.emitLog("error", fmt.Sprintf("[%d] 代理配置错误: %s", workerID, err.Error()))
		return
	}
	dialer.SetOptions(splitRegions(cfg.ProxyRegionRotate), cfg.ProxySessTime, parseProxyStyle(cfg.ProxyStyle), cfg.ProxyState, cfg.StickyEnabled())
	if len(splitRegions(cfg.ProxyRegionRotate)) > 0 {
		r.emitLog("info", fmt.Sprintf("[%d] 代理区域轮换已启用: %s（风格=%s, sessTime=%d 分钟, sticky=%v）", workerID, cfg.ProxyRegionRotate, cfg.ProxyStyle, cfg.ProxySessTime, cfg.StickyEnabled()))
	}
	// 节点池模式：用订阅里存活的节点替代固定代理，每个节点一个出口 IP。
	var watcher *poolWatcher
	if cfg.ProxyPoolUse {
		pool, err := LoadProxyPool(proxyPoolPath(cfg))
		if err != nil {
			r.emitLog("error", fmt.Sprintf("[%d] 读取节点池失败（%s），回退到固定代理", workerID, err.Error()))
		} else {
			specs := nodesToDialerSpecs(pool.Nodes)
			if len(specs) == 0 {
				r.emitLog("error", fmt.Sprintf("[%d] 节点池里没有存活节点，请先「拉取订阅 + 节点测活」", workerID))
			} else {
				dialer.SetPoolNodes(specs, workerID-1)
				r.emitLog("info", fmt.Sprintf("[%d] 节点池已启用：%d 个存活节点，每次尝试轮换出口 IP", workerID, len(specs)))
			}
		}
		watcher = &poolWatcher{path: proxyPoolPath(cfg)}
	}

	var bridge *LocalHTTPBridge
	bridgeAddr := ""
	if cfg.TicketMode == "cdp" && dialer.Enabled() && !cfg.CaptchaDirect {
		bridge, bridgeAddr, err = StartLocalHTTPBridgeWithDialer(dialer)
		if err != nil {
			r.emitLog("error", fmt.Sprintf("[%d] 启动本地代理桥失败: %s", workerID, err.Error()))
			return
		}
		defer bridge.Close()
	}

	var ticketProvider TicketProvider
	if cfg.TicketMode == "external" {
		ticketProvider = &ExternalTicketProvider{
			Endpoint: cfg.ExternalTicketURL,
			HTTP:     dialer.HTTPClient(60 * time.Second),
		}
	} else {
		ticketProvider = &PureGoCDPTicketProvider{
			ChromePath:  cfg.ChromePath,
			ProxyBridge: bridgeAddr,
			SiteKey:     cfg.RecaptchaSiteKey,
			ProfileRoot: filepath.Join(cfg.OutputDir, "chrome-profiles"),
		}
	}

	for {
		if ctx.Err() != nil {
			return
		}
		r.mu.Lock()
		done := r.summary.Succeeded >= cfg.Count
		r.mu.Unlock()
		if done {
			return
		}

		attempt := int(atomic.AddInt64(&r.attemptSeq, 1))
		res := r.runOne(ctx, cfg, dialer, bridgeAddr, panel, ticketProvider, store, watcher, workerID, attempt)
		r.mu.Lock()
		if res.OK {
			r.summary.Succeeded++
			r.summary.Results = append(r.summary.Results, res)
		} else {
			r.summary.FailedAttempts++
		}
		r.mu.Unlock()
		r.saveSummary(summaryPath)
		r.emitState()

		select {
		case <-ctx.Done():
			return
		case <-time.After(3 * time.Second):
		}
	}
}

func (r *ProtocolRunner) setStage(stage, email string) {
	r.mu.Lock()
	r.summary.CurrentStage = stage
	if email != "" {
		r.summary.CurrentEmail = email
	}
	r.mu.Unlock()
	r.emitState()
}

func (r *ProtocolRunner) runOne(ctx context.Context, cfg Config, dialer *ProxyDialer, bridgeAddr string, panel *PanelClient, tp TicketProvider, store *HotmailPoolStore, watcher *poolWatcher, workerID, attempt int) AccountResult {
	res := AccountResult{
		Attempt:   attempt,
		CreatedAt: time.Now().Format(time.RFC3339),
	}
	// 节点池被刷新后热加载（不中断批量）
	if watcher != nil {
		if specs, ok := watcher.refresh(); ok && len(specs) > 0 && dialer != nil {
			dialer.SetPoolNodes(specs, workerID-1)
			r.emitLog("info", fmt.Sprintf("[%d] 节点池已热更新：%d 个存活节点", workerID, len(specs)))
		}
	}
	if dialer != nil {
		dialer.RotateSession()
	}

	var hotmailAcc *HotmailAccount
	var poolIdx = -1

	if cfg.MailMode == "hotmail" {
		acc, idx, err := store.Claim()
		if err != nil {
			res.Error = "领取 Hotmail 邮箱失败: " + err.Error()
			r.emitLog("error", fmt.Sprintf("[%d] %s", workerID, res.Error))
			if errors.Is(err, ErrHotmailPoolExhausted) {
				r.Stop()
			}
			return res
		}
		hotmailAcc = acc
		poolIdx = idx
		res.Email = hotmailAcc.Email
	} else {
		res.Email = fmt.Sprintf("%s%s@%s", cfg.MailPrefix, randomAlnum(8), cfg.MailDomain)
	}

	finalizeHotmail := func(status, lastErr string) {
		if cfg.MailMode == "hotmail" {
			refreshToken := ""
			if hotmailAcc != nil {
				refreshToken = hotmailAcc.RefreshToken
			}
			store.Finalize(poolIdx, res.Email, status, lastErr, refreshToken)
		}
	}

	r.setStage("申请设备码", res.Email)
	signIn, err := panel.StartPluginSignIn(ctx)
	if err != nil {
		// Try re-login once
		_ = panel.Login(ctx)
		signIn, err = panel.StartPluginSignIn(ctx)
		if err != nil {
			res.Error = "申请设备码失败: " + err.Error()
			r.emitLog("error", res.Error)
			finalizeHotmail("available", res.Error)
			return res
		}
	}
	res.UserCode = signIn.Code
	res.SignInID = signIn.ID
	loginRedirect := "/oauth-authorize?user_code=" + url.QueryEscape(signIn.Code)
	if parsedURL, err := url.Parse(signIn.URL); err == nil && parsedURL != nil {
		q := parsedURL.Query()
		if code := strings.TrimSpace(q.Get("user_code")); code != "" {
			loginRedirect = "/oauth-authorize?user_code=" + url.QueryEscape(code)
		}
	}

	r.setStage("获取风控票据", res.Email)
	ticketRes, err := tp.AcquireTicket(ctx, res.Email, signIn.Code)
	if err != nil {
		res.Error = "获取票据失败: " + err.Error()
		r.emitLog("error", res.Error)
		_ = os.WriteFile(filepath.Join(cfg.OutputDir, "last-ticket-error.txt"), []byte(res.Error), 0o600)
		finalizeHotmail("available", res.Error)
		return res
	}

	var mm *MiniMaxClient
	if cfg.CaptchaDirect {
		mm = NewMiniMaxClient(nil, "", cfg.SignatureSalt, cfg.YySuffix)
	} else {
		mm = NewMiniMaxClient(dialer, bridgeAddr, cfg.SignatureSalt, cfg.YySuffix)
	}
	mm.SetTimezone(cfg.TimezoneID)
	mm.SetCookies(ticketRes.Cookies)
	if ticketRes.UUID != "" {
		mm.uuid = ticketRes.UUID
	}
	cookieNames := make([]string, 0, len(ticketRes.Cookies))
	for name := range ticketRes.Cookies {
		cookieNames = append(cookieNames, name)
	}
	r.emitLog("info", fmt.Sprintf("票据长度=%d cookies=%v uuid=%v", len(ticketRes.Ticket), cookieNames, ticketRes.UUID != ""))
	if ticketRes.Cookies["_abck"] == "" {
		r.emitLog("info", "警告：当前会话缺少 _abck，发码可能被拒")
	}

	r.setStage("注册设备", res.Email)
	// 始终走 register-device，拿到服务端 deviceIDStr；页面 localStorage 里的硬件信息不能直接当设备 ID。
	if err := mm.RegisterDevice(ctx); err != nil {
		res.Error = "注册设备失败: " + err.Error()
		r.emitLog("error", res.Error)
		finalizeHotmail("available", res.Error)
		return res
	}

	r.setStage("检查账号状态", res.Email)
	code, msg, err := mm.CheckAccount(ctx, res.Email, loginRedirect)
	if err != nil {
		res.Error = "检查账号失败: " + err.Error()
		r.emitLog("error", res.Error)
		finalizeHotmail("available", res.Error)
		return res
	}
	if code != 1200056 && code != 0 {
		r.emitLog("info", fmt.Sprintf("账号预检返回 code=%d (%s)，继续尝试发码", code, msg))
	}
	if code == 1200058 || code == 1200059 {
		r.emitLog("info", fmt.Sprintf("[%s] 预检提示该邮箱已有密码/已注册(code=%d)，建密大概率被拒，属号池残留账号", res.Email, code))
	}

	r.setStage("发送邮箱验证码", res.Email)
	sendStartedAt := time.Now()
	sendErr := mm.SendCode(ctx, res.Email, ticketRes.Ticket)
	for attempt := 1; attempt <= 2 && sendErr != nil; attempt++ {
		needRetry := strings.Contains(sendErr.Error(), "1200063")
		if !needRetry {
			break
		}
		r.emitLog("info", fmt.Sprintf("[%s] 人机验证被拒，第 %d 次重取票据重试", res.Email, attempt))
		if ctx.Err() != nil {
			break
		}
		ticket2, terr := tp.AcquireTicket(ctx, res.Email, signIn.Code)
		if terr == nil && ticket2 != nil && strings.TrimSpace(ticket2.Ticket) != "" {
			mm.SetCookies(ticket2.Cookies)
			sendErr = mm.SendCode(ctx, res.Email, ticket2.Ticket)
			continue
		}
		break
	}
	if sendErr != nil {
		res.Error = sendErr.Error()
		r.emitLog("error", fmt.Sprintf("[%s] 发码失败: %s", res.Email, res.Error))
		_ = os.WriteFile(filepath.Join(cfg.OutputDir, "last-send-code-error.txt"), []byte(res.Error), 0o600)
		finalizeHotmail("available", res.Error)
		return res
	}

	r.setStage("轮询邮箱验证码", res.Email)
	var verifyCode string
	if cfg.MailMode == "hotmail" {
		var mailHTTP *http.Client
		if cfg.MailViaProxy && dialer != nil && dialer.Enabled() {
			mailHTTP = dialer.HTTPClient(25 * time.Second)
		} else {
			mailHTTP = &http.Client{
				Timeout: 35 * time.Second,
				Transport: &http.Transport{
					TLSHandshakeTimeout:   25 * time.Second,
					ResponseHeaderTimeout: 25 * time.Second,
					DisableKeepAlives:     true,
				},
			}
		}
		verifyCode, err = PollHotmailCode(ctx, mailHTTP, hotmailAcc, sendStartedAt.Add(-90*time.Second), 90*time.Second)
		if err != nil {
			res.Error = err.Error()
			r.emitLog("error", fmt.Sprintf("[%s] Hotmail 取码失败: %s", res.Email, res.Error))
			if strings.Contains(res.Error, "AADSTS70000") || strings.Contains(res.Error, "grant is expired") {
				finalizeHotmail("skipped", res.Error)
			} else {
				finalizeHotmail("available", res.Error)
			}
			return res
		}
	} else {
		verifyCode, err = PollIMAPCode(ctx, cfg.ImapHost, cfg.ImapPort, cfg.ImapUser, cfg.ImapPassword, res.Email, 90*time.Second)
		if err != nil {
			res.Error = err.Error()
			r.emitLog("error", fmt.Sprintf("[%s] IMAP 取码失败: %s", res.Email, res.Error))
			return res
		}
	}

	r.setStage("创建密码并登录", res.Email)
	if strings.TrimSpace(cfg.FixedPassword) != "" {
		r.emitLog("info", fmt.Sprintf("[%s] 使用固定密码模式（长度 %d）；否则为随机强密码", res.Email, len(cfg.FixedPassword)))
	}
	// 1200058 (Password set but incorrect) 真因排查：
	//  a) 号池残留——邮箱之前已在 MiniMax 注册过并设过密码，建密接口拒绝覆盖 → 该邮箱应判 skipped（不可复用）。
	//  b) 出口 IP 被风控误判 → 换 IP 重试可能恢复。
	//  策略：固定密码模式只换 IP 不换密码；随机模式每轮换新密码 + 新 IP，最多 2 轮。
	//  若预检已是 1200058/1200059（上面有日志），则 1 次失败后直接判残留。
	loginErr := error(nil)
	for attempt := 1; ; attempt++ {
		pwd := randomPassword()
		if fp := strings.TrimSpace(cfg.FixedPassword); fp != "" {
			pwd = fp
		}
		loginErr = mm.CreatePasswordLogin(ctx, res.Email, pwd, verifyCode, loginRedirect)
		if loginErr == nil {
			break
		}
		if !strings.Contains(loginErr.Error(), "1200058") || attempt >= 2 {
			break
		}
		if ctx.Err() != nil {
			break
		}
		r.emitLog("info", fmt.Sprintf("[%s] 1200058(密码被拒)，第 %d 次重试：换出口 IP（固定密码模式不换密码）", res.Email, attempt))
		if dialer != nil && dialer.Enabled() {
			dialer.RotateSession()
			mm.http = NewMiniMaxClient(dialer, bridgeAddr, cfg.SignatureSalt, cfg.YySuffix).http
		}
		time.Sleep(2 * time.Second)
	}
	if loginErr != nil {
		if strings.Contains(loginErr.Error(), "1200058") {
			// 1200058 = the mailbox already has a MiniMax password (residual account), downgrade to loginType=21 code-only login to recover
			r.emitLog("info", fmt.Sprintf("[%s] 1200058 -> residual-account recovery: using code-only login (loginType=21)", res.Email))
			codeErr := mm.CodeOnlyLogin(ctx, res.Email, verifyCode, loginRedirect)
			if codeErr != nil && strings.Contains(codeErr.Error(), "1200059") {
				r.emitLog("info", fmt.Sprintf("[%s] verification code expired, re-sending code then retry 21", res.Email))
				if sendErr2 := mm.SendCode(ctx, res.Email, ticketRes.Ticket); sendErr2 == nil {
					var mailHTTP *http.Client
					if cfg.MailViaProxy && dialer != nil && dialer.Enabled() {
						mailHTTP = dialer.HTTPClient(25 * time.Second)
					} else {
						mailHTTP = &http.Client{
							Timeout: 35 * time.Second,
							Transport: &http.Transport{
								TLSHandshakeTimeout:   25 * time.Second,
								ResponseHeaderTimeout: 25 * time.Second,
								DisableKeepAlives:     true,
							},
						}
					}
					if newCode, perr := PollHotmailCode(ctx, mailHTTP, hotmailAcc, time.Now().Add(-90*time.Second), 90*time.Second); perr == nil {
						codeErr = mm.CodeOnlyLogin(ctx, res.Email, newCode, loginRedirect)
					} else {
						codeErr = perr
					}
				} else {
					codeErr = fmt.Errorf("re-send verification code failed: %w", sendErr2)
				}
			}
			if codeErr == nil {
				loginErr = nil
				r.emitLog("info", fmt.Sprintf("[%s] residual-account recovery successful (code-only login)", res.Email))
			} else {
				res.Error = fmt.Sprintf("create-password 1200058 and code-only login fallback failed: %s", codeErr.Error())
				r.emitLog("error", fmt.Sprintf("[%s] %s", res.Email, res.Error))
				finalizeHotmail("skipped", res.Error)
				return res
			}
		}
		if loginErr != nil {
			res.Error = loginErr.Error()
			r.emitLog("error", fmt.Sprintf("[%s] %s", res.Email, res.Error))
			finalizeHotmail("available", res.Error)
			return res
		}
	}


	r.setStage("设备授权审批", res.Email)
	panel.CeremonyLock()
	if err := mm.AuthorizeDevice(ctx, signIn.Code); err != nil {
		panel.CeremonyUnlock()
		res.Error = err.Error()
		r.emitLog("error", fmt.Sprintf("[%d] [%s] 授权失败: %s", workerID, res.Email, res.Error))
		finalizeHotmail("available", res.Error)
		return res
	}
	r.emitLog("info", fmt.Sprintf("[%d] [%s] 授权成功，等待面板落库", workerID, res.Email))

	r.setStage("等待面板绑定完成", res.Email)
	beforeUsers, _ := panel.ListMiniMaxUsers(ctx)
	// 多路并行时面板落库排队会更久，等待窗口放宽到 180s（锁内串行）
	pollResp, providersUser, err := panel.WaitForBinding(ctx, signIn.ID, beforeUsers, 180*time.Second)
	panel.CeremonyUnlock()
	if err != nil {
		res.Error = err.Error()
		r.emitLog("error", fmt.Sprintf("[%d] [%s] 面板绑定失败: %s", workerID, res.Email, res.Error))
		finalizeHotmail("available", res.Error)
		return res
	}
	res.OK = true
	if pollResp != nil && pollResp.User != "" {
		res.ProviderName = pollResp.User
	} else {
		res.ProviderName = providersUser
	}
	finalizeHotmail("used", "")
	r.emitLog("info", fmt.Sprintf("注册并绑定成功: %s -> %s", res.Email, res.ProviderName))
	r.dumpSession(cfg, res.Email, mm.CookieSnapshot())
	r.dailySignin(ctx, cfg, mm, &res)
	return res
}

func (r *ProtocolRunner) dumpSession(cfg Config, email string, cookies map[string]string) {
	dir := filepath.Join(cfg.OutputDir, "sessions")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return
	}
	safe := strings.NewReplacer("@", "_at_", ":", "_", "/", "_", "\\", "_").Replace(email)
	data, err := json.MarshalIndent(map[string]any{
		"email":     email,
		"cookies":   cookies,
		"createdAt": time.Now().Format(time.RFC3339),
	}, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(dir, safe+".json"), data, 0o600)
}

// dailySignin 在注册成功后，用自己的 OAuth 设备码换 access token 并完成每日签到。
func (r *ProtocolRunner) dailySignin(ctx context.Context, cfg Config, mm *MiniMaxClient, res *AccountResult) {
	if !cfg.SigninAfterRegister {
		return
	}
	r.setStage("每日签到领取积分", res.Email)
	signCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	flow, err := mm.StartOwnDeviceFlow(signCtx)
	if err != nil {
		r.emitLog("error", fmt.Sprintf("[%s] 签到设备码申请失败: %s", res.Email, err.Error()))
		res.Signin = "device_error"
		return
	}
	if err := mm.AuthorizeDevice(signCtx, flow.UserCode); err != nil {
		r.emitLog("error", fmt.Sprintf("[%s] 签到授权失败: %s", res.Email, err.Error()))
		res.Signin = "authorize_error"
		return
	}
	access, refresh, err := mm.ExchangeDeviceToken(signCtx, flow)
	if err != nil {
		r.emitLog("error", fmt.Sprintf("[%s] 签到令牌交换失败: %s", res.Email, err.Error()))
		res.Signin = "token_error"
		return
	}
	userID := mm.MCodeUserID(signCtx, access)
	result, err := mm.CheckinWithToken(signCtx, access, userID)
	if err != nil {
		r.emitLog("error", fmt.Sprintf("[%s] 签到失败: %s", res.Email, err.Error()))
		res.Signin = "checkin_error"
		r.saveCheckinCredential(cfg, res.Email, userID, access, refresh, 0, "error")
		return
	}
	if result.Points > 0 {
		res.Signin = fmt.Sprintf("+%d", result.Points)
	} else if result.Claimed {
		res.Signin = "claimed"
	} else {
		res.Signin = "already"
	}
	r.emitLog("info", fmt.Sprintf("[%s] 签到完成: 今天已领取=%v 积分=%d（%s）", res.Email, result.Claimed, result.Points, result.RawClaim))
	r.saveCheckinCredential(cfg, res.Email, userID, access, refresh, result.Points, res.Signin)
}

// saveCheckinCredential 保存 token，便于后续批量补签（无需再次收验证码）。
func (r *ProtocolRunner) saveCheckinCredential(cfg Config, email, userID, access, refresh string, points int, status string) {
	dir := filepath.Join(cfg.OutputDir, "checkin")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return
	}
	safe := strings.NewReplacer("@", "_at_", ":", "_", "/", "_", "\\", "_").Replace(email)
	data, err := json.MarshalIndent(map[string]any{
		"email":        email,
		"userID":       userID,
		"accessToken":  access,
		"refreshToken": refresh,
		"points":       points,
		"status":       status,
		"updatedAt":    time.Now().Format(time.RFC3339),
	}, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(dir, safe+".json"), data, 0o600)
}

func trimForLog(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 200 {
		return s[:200] + "..."
	}
	return s
}
