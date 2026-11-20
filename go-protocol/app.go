package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
			points := 0
			for _, r := range s.Results {
				if v, err := strconv.Atoi(strings.TrimPrefix(strings.TrimSpace(r.Signin), "+")); err == nil && v > 0 {
					points += v
				}
			}
			emitEvent("batch:progress", ProgressLite{
				Target:         s.Target,
				Succeeded:      s.Succeeded,
				FailedAttempts: s.FailedAttempts,
				CurrentStage:   s.CurrentStage,
				CurrentEmail:   s.CurrentEmail,
				Running:        s.Running,
				UpdatedAt:      s.UpdatedAt,
				Points:         points,
			})
		},
		func(e StepEvent) {
			emitEvent("batch:step", e)
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

// ProgressLite 是轻量进度快照：不含 results 列表，避免上千行结果随每次事件推送到前端。
type ProgressLite struct {
	Target         int    `json:"target"`
	Succeeded      int    `json:"succeeded"`
	FailedAttempts int    `json:"failedAttempts"`
	CurrentStage   string `json:"currentStage"`
	CurrentEmail   string `json:"currentEmail"`
	Running        bool   `json:"running"`
	UpdatedAt      string `json:"updatedAt"`
	Points         int    `json:"points"`
	FirstAt        string `json:"firstAt"`
	LastAt         string `json:"lastAt"`
}

// GetProgressLite 返回轻量进度快照。
func (a *App) GetProgressLite() ProgressLite {
	s := a.runner.Summary()
	points := 0
	firstAt, lastAt := "", ""
	for _, r := range s.Results {
		if v, err := strconv.Atoi(strings.TrimPrefix(strings.TrimSpace(r.Signin), "+")); err == nil && v > 0 {
			points += v
		}
		if r.CreatedAt == "" {
			continue
		}
		if firstAt == "" || r.CreatedAt < firstAt {
			firstAt = r.CreatedAt
		}
		if lastAt == "" || r.CreatedAt > lastAt {
			lastAt = r.CreatedAt
		}
	}
	return ProgressLite{
		Target:         s.Target,
		Succeeded:      s.Succeeded,
		FailedAttempts: s.FailedAttempts,
		CurrentStage:   s.CurrentStage,
		CurrentEmail:   s.CurrentEmail,
		Running:        s.Running,
		UpdatedAt:      s.UpdatedAt,
		Points:         points,
		FirstAt:        firstAt,
		LastAt:         lastAt,
	}
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

// ImportHotmailText 直接导入前端读取的大文本内容（桌面版文件导入走这条通道，
// 不依赖服务端 /api/upload，避免大文件粘贴被截断）。
func (a *App) ImportHotmailText(cfg Config, raw string) (HotmailPoolStats, error) {
	cfg.applyDefaults()
	raw = strings.TrimPrefix(raw, "\ufeff")
	if strings.TrimSpace(raw) == "" {
		return HotmailPoolStats{}, errors.New("导入文本为空")
	}
	stats, added, err := ImportHotmailRaw(cfg.HotmailPool, raw)
	if err != nil {
		return stats, err
	}
	lines := strings.Count(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	a.emitBatchLog("info", fmt.Sprintf("导入 Hotmail 完成：文本 %d 行，新增 %d，总计 %d，可用 %d",
		lines+1, added, stats.Total, stats.Available))
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

// MailboxOption 是号池邮箱的检索结果，带状态供前端做分类与快速选择。
type MailboxOption struct {
	Email  string `json:"email"`
	Status string `json:"status"`
	Note   string `json:"note"`
}

// SearchMailboxes 在号池中按关键字检索邮箱（大小写不敏感，支持前缀/子串匹配）。
// 返回结果按"前缀命中 > 子串命中"排序，便于前端做快速选择。
func (a *App) SearchMailboxes(cfg Config, query string, limit int) ([]MailboxOption, error) {
	cfg.applyDefaults()
	if limit <= 0 {
		limit = 50
	}
	pool, err := LoadHotmailPool(cfg.HotmailPool)
	if err != nil {
		return nil, err
	}
	q := strings.ToLower(strings.TrimSpace(query))
	out := make([]MailboxOption, 0, limit)
	for _, acc := range pool.Accounts {
		if acc.Email == "" {
			continue
		}
		note := acc.LastError
		if q != "" {
			lower := strings.ToLower(acc.Email)
			if !strings.HasPrefix(lower, q) && !strings.Contains(lower, q) {
				continue
			}
	}
		out = append(out, MailboxOption{Email: acc.Email, Status: acc.Status, Note: note})
	if len(out) >= limit {
			break
		}
	}
	// 前缀命中排前面
	if q != "" {
		sort.SliceStable(out, func(i, j int) bool {
			return strings.HasPrefix(strings.ToLower(out[i].Email), q) && !strings.HasPrefix(strings.ToLower(out[j].Email), q)
		})
	}
	return out, nil
}

// ResultsPage 是已绑定账号的分页结果。
type ResultsPage struct {
	Total  int             `json:"total"`
	Offset int             `json:"offset"`
	Limit  int             `json:"limit"`
	Rows   []AccountResult `json:"rows"`
}

// GetResultsPage 从 summary.json 读取已绑定账号，支持关键字过滤与排序后分页，
// 避免前端一次渲染上千行。
func (a *App) GetResultsPage(cfg Config, offset, limit int, query, sortBy string) (ResultsPage, error) {
	cfg.applyDefaults()
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	raw, err := os.ReadFile(filepath.Join(cfg.OutputDir, "summary.json"))
	if err != nil {
	return ResultsPage{Rows: []AccountResult{}}, nil
	}
	var sum BatchSummary
	if err := json.Unmarshal(raw, &sum); err != nil {
	return ResultsPage{Rows: []AccountResult{}}, nil
	}
	rows := sum.Results
	if rows == nil {
		rows = []AccountResult{}
	}
	q := strings.ToLower(strings.TrimSpace(query))
	if q != "" {
		filtered := make([]AccountResult, 0, len(rows))
		for _, r := range rows {
			hay := strings.ToLower(r.Email + " " + r.ProviderName + " " + r.UserCode)
			if strings.Contains(hay, q) {
				filtered = append(filtered, r)
			}
		}
		rows = filtered
	}
	switch sortBy {
	case "email":
		sort.SliceStable(rows, func(i, j int) bool { return rows[i].Email < rows[j].Email })
	case "-email":
		sort.SliceStable(rows, func(i, j int) bool { return rows[i].Email > rows[j].Email })
	case "provider":
		sort.SliceStable(rows, func(i, j int) bool { return rows[i].ProviderName < rows[j].ProviderName })
	case "-provider":
		sort.SliceStable(rows, func(i, j int) bool { return rows[i].ProviderName > rows[j].ProviderName })
	case "-time": // time asc
		sort.SliceStable(rows, func(i, j int) bool { return rows[i].CreatedAt < rows[j].CreatedAt })
	default: // time desc
		sort.SliceStable(rows, func(i, j int) bool { return rows[i].CreatedAt > rows[j].CreatedAt })
	}
	total := len(rows)
	if offset < 0 {
		offset = 0
	}
	if offset >= total {
	return ResultsPage{Total: total, Offset: offset, Limit: limit, Rows: []AccountResult{}}, nil
	}
	end := offset + limit
	if end > total {
	end = total
	}
	return ResultsPage{Total: total, Offset: offset, Limit: limit, Rows: rows[offset:end]}, nil
}

func (a *App) ClearHistory(cfg Config) error {
	if a.runner.IsRunning() {
		return fmt.Errorf("请先停止当前运行任务")
	}
	cfg.applyDefaults()
	_ = os.Remove(filepath.Join(cfg.OutputDir, "summary.json"))
	return nil
}
