package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type App struct {
	ctx        context.Context
	runner     *ProtocolRunner
	preflightM sync.Mutex
	preflightC context.CancelFunc
	supM       sync.Mutex
	supStop    chan struct{}
	supRunning bool
}

func NewApp() *App {
	a := &App{}
	a.runner = NewProtocolRunner(
		func(level, msg string) {
			a.emitBatchLog(level, msg)
		},
		func(s BatchSummary) {
			emitEvent("batch:progress", s)
		},
	)
	return a
}

// emitBatchLog 统一的批量日志事件发射。
func (a *App) emitBatchLog(level, msg string) {
	emitEvent("batch:log", map[string]string{
		"time":    time.Now().Format("15:04:05"),
		"level":   level,
		"message": msg,
	})
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) shutdown(_ context.Context) {
	a.runner.Stop()
	a.preflightM.Lock()
	if a.preflightC != nil {
		a.preflightC()
	}
	a.preflightM.Unlock()
	a.stopSupervised()
}

// StartSupervised 监督批量：异常自动重启续跑，直到成功数达标、号池耗尽或点「停止」。
func (a *App) StartSupervised(cfg Config) error {
	cfg.applyDefaults()
	if err := SaveConfig(cfg); err != nil {
		return err
	}
	a.supM.Lock()
	if a.supRunning {
		a.supM.Unlock()
		return fmt.Errorf("监督批量已在运行中")
	}
	a.supRunning = true
	a.supStop = make(chan struct{})
	stopCh := a.supStop
	a.supM.Unlock()

	go a.runSupervisedLoop(cfg, stopCh)
	return nil
}

// runSupervisedLoop 用 App 主 runner 驱动监督循环（而非自建临时 runner），
// 使右上角"实时运行状态"卡片、"已成功绑定账号"表格与 GetProgress 完全同步。
// 每轮 Start 一次，summary.json 持久化自动续跑；达标/号池耗尽/手动停止时收工。
func (a *App) runSupervisedLoop(cfg Config, stopCh <-chan struct{}) {
	for round := 1; ; round++ {
		if stopWait(stopCh, 0) {
			a.stopSupervised()
			return
		}
		if n, err := RecycleReserved(cfg.HotmailPool); err == nil && n > 0 {
			a.emitBatchLog("info", fmt.Sprintf("[supervised] 第 %d 轮：回收残留 reserved 账号 %d 个", round, n))
		}
		if err := a.runner.Start(cfg); err != nil {
			a.emitBatchLog("error", fmt.Sprintf("[supervised] 第 %d 轮启动失败: %v，60s 后重试", round, err))
			if stopWait(stopCh, 60*time.Second) {
				a.stopSupervised()
				return
			}
			continue
		}

		// 等本轮自然结束或收到停止信号
		for {
			select {
			case <-stopCh:
				a.runner.Stop()
				a.stopSupervised()
				return
			default:
			}
			if !a.runner.IsRunning() {
				break
			}
			time.Sleep(3 * time.Second)
		}

		sum := a.runner.Summary()
		a.emitBatchLog("info", fmt.Sprintf("[supervised] 第 %d 轮结束：成功 %d/%d，失败尝试 %d", round, sum.Succeeded, sum.Target, sum.FailedAttempts))
		if sum.Succeeded >= sum.Target {
			a.emitBatchLog("info", fmt.Sprintf("达成目标：成功 %d/%d，监督循环结束", sum.Succeeded, sum.Target))
			a.stopSupervised()
			return
		}
		stats, err := loadPoolStats(cfg.HotmailPool)
		if err == nil && stats.Available+stats.Reserved == 0 {
			a.emitBatchLog("info", "[supervised] 号池已耗尽，停止监督循环")
			a.stopSupervised()
			return
		}
		if stopWait(stopCh, 20*time.Second) {
			a.stopSupervised()
			return
		}
		a.emitBatchLog("info", "[supervised] 20s 后自动重启继续…")
	}
}

// stopWait 在 d 超时内等待 stopCh 关闭；d=0 时非阻塞检查一次。
func stopWait(stopCh <-chan struct{}, d time.Duration) bool {
	if d <= 0 {
		select {
		case <-stopCh:
			return true
		default:
			return false
		}
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-stopCh:
		return true
	case <-t.C:
		return false
	}
}

// StopSupervised 停止监督批量。
func (a *App) StopSupervised() {
	a.stopSupervised()
}

func (a *App) stopSupervised() {
	a.supM.Lock()
	defer a.supM.Unlock()
	if a.supRunning && a.supStop != nil {
		select {
		case <-a.supStop:
		default:
			close(a.supStop)
		}
	}
	a.supRunning = false
}

func (a *App) GetConfig() (Config, error) {
	return LoadConfig()
}

func (a *App) SaveConfig(cfg Config) (Config, error) {
	// hotmailRaw 只是粘贴缓冲（可达数百 KB），不落盘，避免配置膨胀与回填误判
	cfg.HotmailRaw = ""
	if err := SaveConfig(cfg); err != nil {
		return cfg, err
	}
	return LoadConfig()
}

func (a *App) GetProgress() BatchSummary {
	return a.runner.Summary()
}

func (a *App) TestPanel(cfg Config) (string, error) {
	cfg.applyDefaults()
	dialer, err := NewProxyDialer(cfg.ProxyURL)
	if err != nil {
		return "", err
	}
	panel, err := NewPanelClient(cfg.PanelBase, cfg.PanelKey, dialer, cfg.ProxyDirectHosts)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := panel.Login(ctx); err != nil {
		return "", err
	}
	providers, err := panel.CheckProviders(ctx)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("面板连接正常，已读取 %d 个供应商配置", len(providers)), nil
}

func (a *App) ImportHotmail(cfg Config) (HotmailPoolStats, error) {
	cfg.applyDefaults()
	stats, added, err := ImportHotmailRaw(cfg.HotmailPool, cfg.HotmailRaw)
	if err != nil {
		return stats, err
	}
	a.emitBatchLog("info", fmt.Sprintf("导入 Hotmail 完成：新增 %d，总计 %d，可用 %d", added, stats.Total, stats.Available))
	return stats, nil
}

// ImportHotmailFile 从服务器本地文件导入号池（大文本推荐路径，避免粘贴截断）。
func (a *App) ImportHotmailFile(cfg Config, path string) (HotmailPoolStats, error) {
	cfg.applyDefaults()
	raw, err := os.ReadFile(strings.TrimSpace(path))
	if err != nil {
		return HotmailPoolStats{}, fmt.Errorf("读取导入文件失败: %w", err)
	}
	stats, added, err := ImportHotmailRaw(cfg.HotmailPool, string(raw))
	if err != nil {
		return stats, err
	}
	lines := strings.Count(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n")
	a.emitBatchLog("info", fmt.Sprintf("\u5bfc\u5165 Hotmail \u5b8c\u6210\uff1a\u6587\u4ef6 %d \u884c\uff0c\u65b0\u589e %d\uff0c\u603b\u8ba1 %d\uff0c\u53ef\u7528 %d", lines, added, stats.Total, stats.Available))
	return stats, nil
}

func (a *App) GetHotmailStats(cfg Config) (HotmailPoolStats, error) {
	cfg.applyDefaults()
	p, err := LoadHotmailPool(cfg.HotmailPool)
	if err != nil {
		return HotmailPoolStats{}, err
	}
	return p.Stats(), nil
}

// ImportProxySubscription 下载订阅并写入本地节点池文件。
func (a *App) ImportProxySubscription(cfg Config) (ProxyPoolStats, error) {
	cfg.applyDefaults()
	hasLocal := len(parseLocalPorts(cfg.ProxyLocalPorts)) > 0
	if strings.TrimSpace(cfg.ProxySubscriptionURL) == "" && !hasLocal {
		return ProxyPoolStats{}, errors.New("订阅地址不能为空")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	var nodes []ProxyNodeInfo
	var err error
	if strings.TrimSpace(cfg.ProxySubscriptionURL) != "" {
		nodes, err = FetchProxySubscription(ctx, cfg.ProxySubscriptionURL)
	}
	if err != nil {
		// 订阅拉取失败也允许只用本地端口池
		if len(parseLocalPorts(cfg.ProxyLocalPorts)) == 0 {
			return ProxyPoolStats{}, err
		}
		nodes = nil
	}
	// 本地端口池（sing-box 等内核暴露的一端口一出口）
	local := localPortNodes(parseLocalPorts(cfg.ProxyLocalPorts))
	existing := map[string]bool{}
	for _, n := range nodes {
		existing[n.Server+":"+itoa(n.Port)] = true
	}
	for _, n := range local {
		key := n.Server + ":" + itoa(n.Port)
		if existing[key] {
			continue
		}
		existing[key] = true
		nodes = append(nodes, n)
	}
	path := proxyPoolPath(cfg)
	pool := ProxyPool{Source: cfg.ProxySubscriptionURL, Nodes: nodes}
	// 保留已有节点的测活结果（按 server:port 匹配）
	if old, err := LoadProxyPool(path); err == nil {
		prev := map[string]ProxyNodeInfo{}
		for _, n := range old.Nodes {
			prev[n.Server+":"+itoa(n.Port)] = n
		}
		for i := range pool.Nodes {
			if p, ok := prev[pool.Nodes[i].Server+":"+itoa(pool.Nodes[i].Port)]; ok {
				pool.Nodes[i].Alive = p.Alive
				pool.Nodes[i].ExitIP = p.ExitIP
				pool.Nodes[i].Latency = p.Latency
				pool.Nodes[i].LastChk = p.LastChk
			}
		}
	}
	if err := SaveProxyPool(path, pool); err != nil {
		return ProxyPoolStats{}, err
	}
	stats := pool.Stats()
	a.emitBatchLog("info", fmt.Sprintf("订阅拉取完成：%d 个节点已写入 %s", stats.Total, path))
	return stats, nil
}

// CheckProxyPool 对节点池做并发测活，并写回文件。
func (a *App) CheckProxyPool(cfg Config) (ProxyPoolStats, error) {
	cfg.applyDefaults()
	path := proxyPoolPath(cfg)
	pool, err := LoadProxyPool(path)
	if err != nil {
		return ProxyPoolStats{}, err
	}
	if len(pool.Nodes) == 0 {
		return ProxyPoolStats{}, errors.New("节点池为空，请先拉取订阅")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	a.emitBatchLog("info", fmt.Sprintf("开始测活：%d 个节点，%d 并发...", len(pool.Nodes), cfg.ProxyPoolWorkers))
	checked := CheckProxyNodes(ctx, pool.Nodes, cfg.ProxyPoolWorkers, cfg.ProxyPoolCheckURL)
	pool.Nodes = checked
	if err := SaveProxyPool(path, pool); err != nil {
		return ProxyPoolStats{}, err
	}
	stats := pool.Stats()
	stats.Checked = len(checked)
	a.emitBatchLog("info", fmt.Sprintf("测活完成：存活 %d / %d", stats.Alive, stats.Total))
	return stats, nil
}

// GetProxyPoolStats 返回当前节点池统计。
func (a *App) GetProxyPoolStats(cfg Config) (ProxyPoolStats, error) {
	cfg.applyDefaults()
	pool, err := LoadProxyPool(proxyPoolPath(cfg))
	if err != nil {
		return ProxyPoolStats{}, err
	}
	return pool.Stats(), nil
}

func itoa(v int) string {
	return strconv.Itoa(v)
}

func (a *App) PreflightHotmail(cfg Config) (HotmailPoolStats, error) {
	cfg.applyDefaults()
	dialer, err := NewProxyDialer(cfg.ProxyURL)
	if err != nil {
		return HotmailPoolStats{}, err
	}
	a.preflightM.Lock()
	if a.preflightC != nil {
		a.preflightC()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	a.preflightC = cancel
	a.preflightM.Unlock()
	defer cancel()

	var preflightDialer *ProxyDialer
	if cfg.MailViaProxy {
		preflightDialer = dialer
	}

	return PreflightHotmailPool(ctx, cfg.HotmailPool, cfg.HotmailPreflightWorkers, preflightDialer, func(p PreflightProgress) {
		emitEvent("hotmail:preflight", p)
	})
}

func (a *App) RemoveSkippedHotmail(cfg Config) (HotmailPoolStats, error) {
	cfg.applyDefaults()
	p, err := LoadHotmailPool(cfg.HotmailPool)
	if err != nil {
		return HotmailPoolStats{}, err
	}
	kept := make([]HotmailAccount, 0, len(p.Accounts))
	for _, acc := range p.Accounts {
		if acc.Status != "skipped" {
			kept = append(kept, acc)
		}
	}
	p.Accounts = kept
	if err := SaveHotmailPool(cfg.HotmailPool, p); err != nil {
		return HotmailPoolStats{}, err
	}
	return p.Stats(), nil
}

func (a *App) StartBatch(cfg Config) error {
	cfg.applyDefaults()
	if err := SaveConfig(cfg); err != nil {
		return err
	}
	return a.runner.Start(cfg)
}

func (a *App) StopBatch() {
	a.runner.Stop()
}

// CheckinAll 对已保存凭据的账号批量补签（每日一次）。
func (a *App) CheckinAll(cfg Config) (string, error) {
	cfg.applyDefaults()
	if a.runner.IsRunning() {
		return "", fmt.Errorf("请先停止批量注册任务，再执行批量补签")
	}
	claimed, already, failed, points := checkinAllAccounts(cfg, func(line string) {
		a.emitBatchLog("info", line)
	})
	return fmt.Sprintf("补签完成：新领取 %d，今日已领 %d，失败 %d，本轮共 %d 积分", claimed, already, failed, points), nil
}

// MailScanAll 批量收信：扫号池所有邮箱，判定 Cursor 注册痕迹并拉取最新收件箱。
func (a *App) MailScanAll(cfg Config) (string, error) {
	cfg.applyDefaults()
	if a.runner.IsRunning() {
		return "", fmt.Errorf("请先停止批量注册任务，再执行批量收信")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	report, err := ScanHotmailInboxes(ctx, cfg.HotmailPool, cfg.HotmailPreflightWorkers, 0, cfg.MailViaProxy, cfg.ProxyURL, func(line string) {
		a.emitBatchLog("info", line)
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("收信扫描完成：扫 %d/%d，Cursor 注册 %d，令牌失效 %d，MiniMax 信件 %d。报告：%s",
		report.Scanned, report.Total, report.CursorRegistered, report.TokenDead, report.WithMiniMaxMails, report.ReportPath), nil
}

// MailInboxView 拉取某邮箱的实时收件箱列表（收件箱+垃圾邮箱）。
func (a *App) MailInboxView(cfg Config, email string) (MailInboxView, error) {
	cfg.applyDefaults()
	ctx := context.Background()
	return ListMailInbox(ctx, cfg.HotmailPool, email, cfg.MailViaProxy, cfg.ProxyURL)
}

// MailMessageView 拉取单封邮件完整正文。
func (a *App) MailMessageView(cfg Config, email, messageID string) (MailMessageFull, error) {
	cfg.applyDefaults()
	ctx := context.Background()
	return GetMailMessage(ctx, cfg.HotmailPool, email, messageID, cfg.MailViaProxy, cfg.ProxyURL)
}

// MailListOptions 返回号池全部邮箱（收件箱下拉框用）。
func (a *App) MailListOptions(cfg Config) ([]string, error) {
	cfg.applyDefaults()
	all, _, err := MailScanEmails(cfg.HotmailPool)
	return all, err
}

func (a *App) ClearHistory(cfg Config) error {
	if a.runner.IsRunning() {
		return fmt.Errorf("请先停止当前运行任务")
	}
	cfg.applyDefaults()
	_ = os.Remove(filepath.Join(cfg.OutputDir, "summary.json"))
	return nil
}
