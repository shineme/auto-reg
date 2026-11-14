package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/sys/windows"
)

type RunStatus struct {
	Running           bool   `json:"running"`
	Phase             string `json:"phase"`
	Message           string `json:"message"`
	Count             int    `json:"count"`
	Done              int    `json:"done"`
	OK                int    `json:"ok"`
	Failed            int    `json:"failed"`
	Attempts          int    `json:"attempts"`
	Current           int    `json:"current"`
	RestartCount      int    `json:"restartCount"`
	RiskProbeCount    int    `json:"riskProbeCount"`
	RiskCooldownUntil string `json:"riskCooldownUntil"`
	BatchID           string `json:"batchId"`
	SupervisorPID     int    `json:"supervisorPid"`
}

type ProgressEvent struct {
	Type                string  `json:"type"`
	BatchID             string  `json:"batchId"`
	Phase               string  `json:"phase"`
	Message             string  `json:"message"`
	Count               int     `json:"count"`
	Done                int     `json:"done"`
	OK                  int     `json:"ok"`
	Failed              int     `json:"failed"`
	Attempts            int     `json:"attempts"`
	Current             int     `json:"current"`
	RestartCount        int     `json:"restartCount"`
	RiskProbeCount      int     `json:"riskProbeCount"`
	RiskCooldownUntil   string  `json:"riskCooldownUntil"`
	Result              *Result `json:"result,omitempty"`
	UpdatedAt           string  `json:"updatedAt"`
	EventID             string  `json:"eventId"`
	SupervisorPID       int     `json:"supervisorPid"`
	SupervisorStartedAt string  `json:"supervisorStartedAt"`
}

type SupervisorStatus struct {
	PID               int    `json:"pid"`
	BatchID           string `json:"batchId"`
	TargetSuccess     int    `json:"targetSuccessCount"`
	StartedAt         string `json:"startedAt"`
	HeartbeatAt       string `json:"heartbeatAt"`
	Phase             string `json:"phase"`
	Running           bool   `json:"running"`
	Succeeded         int    `json:"succeeded"`
	Failed            int    `json:"failed"`
	FailedAttempts    int    `json:"failedAttempts"`
	RestartCount      int    `json:"restartCount"`
	RiskProbeCount    int    `json:"riskProbeCount"`
	RiskCooldownUntil string `json:"riskCooldownUntil"`
	LastExitCode      int    `json:"lastExitCode"`
	Message           string `json:"message"`
	Error             string `json:"error"`
}

type Result struct {
	OK          bool   `json:"ok"`
	Email       string `json:"email"`
	User        string `json:"user"`
	UserCode    string `json:"userCode"`
	SignInID    string `json:"signInId"`
	FlowStatus  string `json:"flowStatus"`
	SignInState string `json:"signInState"`
	Error       string `json:"error"`
	RunDir      string `json:"runDir"`
}

type RunEvent struct {
	Type    string     `json:"type"`
	Level   string     `json:"level"`
	Message string     `json:"message"`
	Status  *RunStatus `json:"status,omitempty"`
	Result  *Result    `json:"result,omitempty"`
	At      string     `json:"at"`
}

type runner struct {
	app       *App
	mu        sync.Mutex
	cancel    context.CancelFunc
	status    RunStatus
	events    []RunEvent
	outputDir string
}

func newRunner(app *App) *runner {
	return &runner{app: app, status: RunStatus{Phase: "idle", Message: "准备就绪"}}
}

func (runner *runner) emit(event RunEvent) {
	event.At = time.Now().Format(time.RFC3339Nano)
	runner.mu.Lock()
	runner.events = append(runner.events, event)
	if len(runner.events) > 500 {
		runner.events = runner.events[len(runner.events)-500:]
	}
	status := runner.status
	runner.mu.Unlock()
	if event.Status != nil {
		status = *event.Status
	}
	runner.emitEvent("run-event", event)
	runner.emitEvent("run-status", status)
}

func (runner *runner) emitEvent(name string, data ...interface{}) {
	if runner.app.ctx == nil {
		return
	}
	runtime.EventsEmit(runner.app.ctx, name, data...)
}

func (runner *runner) log(level, message string) {
	runner.emit(RunEvent{Type: "log", Level: level, Message: message})
}

func (runner *runner) setStatus(status RunStatus) {
	runner.mu.Lock()
	runner.status = status
	runner.mu.Unlock()
	runner.emit(RunEvent{Type: "status", Level: "info", Message: status.Message, Status: &status})
}

func (runner *runner) Start(cfg Config, dryRun bool) error {
	cleanStaleSupervisorState(cfg.OutputDir)
	runner.mu.Lock()
	if runner.cancel != nil {
		runner.mu.Unlock()
		return errors.New("已有任务正在运行")
	}
	if !dryRun {
		if status, err := readSupervisorStatus(cfg.OutputDir); err == nil && status.Running && processAlive(status.PID) {
			runner.mu.Unlock()
			runner.setStatus(RunStatus{
				Running:       true,
				Phase:         status.Phase,
				Message:       fmt.Sprintf("已接管正在运行的守护进程 %d", status.PID),
				Count:         cfg.Count,
				BatchID:       status.BatchID,
				SupervisorPID: status.PID,
			})
			ctx, cancel := context.WithCancel(context.Background())
			runner.mu.Lock()
			runner.cancel = cancel
			runner.mu.Unlock()
			go runner.watchExistingSupervisor(ctx, cfg, status.BatchID, status.PID)
			return nil
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	runner.cancel = cancel
	runner.mu.Unlock()
	go func() {
		defer func() {
			runner.mu.Lock()
			runner.cancel = nil
			status := runner.status
			runner.mu.Unlock()
			runner.emitEvent("run-status", status)
		}()
		if err := runner.execute(ctx, cfg, dryRun); err != nil {
			runner.log("error", err.Error())
			runner.failStatus(err.Error())
		}
	}()
	return nil
}

func (runner *runner) failStatus(message string) {
	runner.mu.Lock()
	status := runner.status
	status.Running = false
	status.Phase = "failed"
	status.Message = message
	runner.status = status
	runner.mu.Unlock()
	runner.emit(RunEvent{Type: "status", Level: "error", Message: message, Status: &status})
}

func (runner *runner) updateStatus(status RunStatus) {
	runner.mu.Lock()
	runner.status = status
	runner.mu.Unlock()
	runner.emitEvent("run-status", status)
}

func (runner *runner) Stop() {
	runner.mu.Lock()
	cancel := runner.cancel
	status := runner.status
	outputDir := runner.outputDir
	job := runner.app.job
	runner.mu.Unlock()
	if job != nil {
		registeredPID := job.PID()
		if registeredPID > 0 {
			terminateProcessTree(registeredPID)
		}
		job.Stop()
	}
	if status.SupervisorPID != 0 {
		terminateProcessTree(status.SupervisorPID)
	}
	if outputDir != "" {
		markSupervisorStopped(outputDir, status.SupervisorPID)
	}
	if cancel != nil {
		cancel()
	}
	runner.setStatus(RunStatus{Running: false, Phase: "stopped", Message: "软件已关闭，后台任务已停止", Count: status.Count, BatchID: status.BatchID})
}

func processAlive(processID int) bool {
	if processID <= 0 {
		return false
	}
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(processID))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(handle)
	var exitCode uint32
	if err := windows.GetExitCodeProcess(handle, &exitCode); err != nil {
		return false
	}
	return exitCode == 259
}

func (runner *runner) watchExistingSupervisor(ctx context.Context, cfg Config, batchID string, supervisorPID int) {
	if batchID == "" {
		batchID = "resume-200"
	}
	defer func() {
		runner.mu.Lock()
		runner.cancel = nil
		runner.mu.Unlock()
	}()
	runner.watchSupervisor(ctx, cfg, batchID, supervisorPID)
}

func (runner *runner) Status() RunStatus {
	runner.mu.Lock()
	defer runner.mu.Unlock()
	return runner.status
}

func (runner *runner) Events() []RunEvent {
	runner.mu.Lock()
	defer runner.mu.Unlock()
	return append([]RunEvent(nil), runner.events...)
}

func (runner *runner) execute(ctx context.Context, cfg Config, dryRun bool) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	runner.mu.Lock()
	runner.outputDir = cfg.OutputDir
	runner.mu.Unlock()
	if err := os.MkdirAll(cfg.OutputDir, 0o755); err != nil {
		return err
	}
	if _, err := exec.LookPath("node"); err != nil {
		return errors.New("未找到 Node.js，请先安装 Node.js 24+")
	}
	if _, err := os.Stat(cfg.ChromePath); err != nil {
		return fmt.Errorf("未找到 Chrome：%w", err)
	}
	scriptName := "minimax-fast-batch.mjs"
	batchID := time.Now().UTC().Format("20060102-150405")
	if !dryRun {
		if _, err := os.Stat(filepath.Join(cfg.ScriptsDir, "minimax-supervisor.mjs")); err != nil {
			return fmt.Errorf("找不到 scripts/minimax-supervisor.mjs：%w", err)
		}
		scriptName = "minimax-supervisor.mjs"
		batchID = "resume-200"
	}
	localProxy, err := runner.app.bridge.Start(cfg.ProxyURL)
	if err != nil {
		return err
	}
	if cfg.ProxyURL != "" {
		runner.log("info", "已启动本地代理桥；上游类型不会写入日志")
	}
	env := append(os.Environ(),
		"MINIMAX_SIGNATURE_SALT="+os.Getenv("MINIMAX_SIGNATURE_SALT"),
		"MINIMAX_YY_SUFFIX="+os.Getenv("MINIMAX_YY_SUFFIX"),
		"MINIMAX_RECAPTCHA_SITE_KEY="+os.Getenv("MINIMAX_RECAPTCHA_SITE_KEY"),
		"PANEL_BASE="+cfg.PanelBase,
		"PANEL_KEY="+cfg.PanelKey,
		"IMAP_HOST="+cfg.ImapHost,
		fmt.Sprintf("IMAP_PORT=%d", cfg.ImapPort),
		"IMAP_USER="+cfg.ImapUser,
		"IMAP_PASSWORD="+cfg.ImapPassword,
		"IMAP_MAILBOX=INBOX",
		"MAIL_DOMAIN="+cfg.MailDomain,
		"MAIL_PREFIX="+cfg.MailPrefix,
		fmt.Sprintf("COUNT=%d", cfg.Count),
		fmt.Sprintf("TARGET_SUCCESS_COUNT=%d", cfg.Count),
		fmt.Sprintf("PREWARM_CONCURRENCY=%d", cfg.PrewarmConcurrency),
		"CHROME_PATH="+cfg.ChromePath,
		"CHROME_PROFILE="+filepath.Join(cfg.OutputDir, "chrome-gui-profile"),
		"BATCH_ROOT="+cfg.OutputDir,
		"BATCH_ID="+batchID,
		fmt.Sprintf("CDP_PORT=%d", cfg.CDPPort),
		"NODE_NO_WARNINGS=1",
	)
	env = append(env, proxyEnv(localProxy)...)
	if dryRun {
		env = append(env, "DRY_RUN=1", "DRY_RUN_USER_CODE=TEST-CODE")
	}
	command := exec.CommandContext(ctx, "node", filepath.Join(cfg.ScriptsDir, scriptName))
	command.Env = env
	command.Dir = detectWorkspace()
	if dryRun {
		stdout, err := command.StdoutPipe()
		if err != nil {
			return err
		}
		stderr, err := command.StderrPipe()
		if err != nil {
			return err
		}
		var scannerGroup sync.WaitGroup
		scannerGroup.Add(2)
		go runner.scanOutput(&scannerGroup, "stdout", stdout, cfg, dryRun)
		go runner.scanOutput(&scannerGroup, "stderr", stderr, cfg, dryRun)
		if err := command.Start(); err != nil {
			return err
		}
		scannerGroup.Wait()
		if err := command.Wait(); err != nil {
			return fmt.Errorf("票据 dry-run 退出：%w", err)
		}
		runner.setStatus(RunStatus{Running: false, Phase: "done", Message: "票据桥接 dry-run 完成", Count: 1, Done: 1, OK: 1})
		return nil
	}
	if !dryRun {
		command.SysProcAttr = &windows.SysProcAttr{
			CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.CREATE_NO_WINDOW | windows.CREATE_SUSPENDED,
		}
		command.SysProcAttr.CreationFlags &^= windows.CREATE_BREAKAWAY_FROM_JOB
		command.Stdout = nil
		command.Stderr = nil
	}
	runner.setStatus(RunStatus{Running: true, Phase: "starting", Message: "正在启动批量注册守护进程", Count: cfg.Count, BatchID: batchID})
	if err := command.Start(); err != nil {
		return err
	}
	if !dryRun {
		if err := runner.app.job.Track(command.Process.Pid); err != nil {
			_ = command.Process.Kill()
			return fmt.Errorf("无法绑定后台进程：%w", err)
		}
		if err := resumeProcess(command.Process.Pid); err != nil {
			_ = command.Process.Kill()
			return fmt.Errorf("无法启动后台进程：%w", err)
		}
	}
	runner.mu.Lock()
	status := runner.status
	status.SupervisorPID = command.Process.Pid
	status.Message = "守护进程已启动，正在恢复历史成功账号"
	runner.status = status
	runner.mu.Unlock()
	runner.emitEvent("run-status", status)
	go runner.watchSupervisor(ctx, cfg, batchID, command.Process.Pid)
	_ = command.Process.Release()
	return nil
}

func terminateProcessTree(processID int) {
	if processID <= 0 {
		return
	}
	command := exec.Command("taskkill.exe", "/PID", fmt.Sprintf("%d", processID), "/T", "/F")
	_ = command.Run()
}

func markSupervisorStopped(outputDir string, processID int) {
	statusPath := filepath.Join(outputDir, "supervisor.status.json")
	statusData, err := os.ReadFile(statusPath)
	if err != nil {
		return
	}
	var status SupervisorStatus
	if err := json.Unmarshal(statusData, &status); err != nil {
		return
	}
	if status.PID != 0 && status.PID != processID {
		return
	}
	status.Running = false
	status.Phase = "stopped"
	status.Message = "软件已关闭，后台任务已停止"
	status.HeartbeatAt = time.Now().UTC().Format(time.RFC3339Nano)
	encoded, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(statusPath, encoded, 0o600)
	_ = os.Remove(filepath.Join(outputDir, "supervisor.lock"))
}

func cleanStaleSupervisorState(outputDir string) {
	status, err := readSupervisorStatus(outputDir)
	if err != nil {
		return
	}
	if status.PID <= 0 || processAlive(status.PID) {
		return
	}
	status.Running = false
	status.Phase = "stopped"
	status.Message = "软件已关闭，后台任务已停止"
	status.HeartbeatAt = time.Now().UTC().Format(time.RFC3339Nano)
	encoded, marshalErr := json.MarshalIndent(status, "", "  ")
	if marshalErr != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(outputDir, "supervisor.status.json"), encoded, 0o600)
	_ = os.Remove(filepath.Join(outputDir, "supervisor.lock"))
}

func (runner *runner) finishFromSummary(summary SupervisorSummary, count int, batchID string, supervisorPID int) {
	phase := "done"
	message := fmt.Sprintf("完成：成功 %d，失败尝试 %d", summary.Succeeded, summary.Failed)
	if summary.Succeeded < count {
		phase = "failed"
		message = fmt.Sprintf("已停止：成功 %d/%d，失败尝试 %d", summary.Succeeded, count, summary.Failed)
	}
	runner.setStatus(RunStatus{
		Running:       false,
		Phase:         phase,
		Message:       message,
		Count:         count,
		Done:          summary.Succeeded,
		OK:            summary.Succeeded,
		Failed:        summary.Failed,
		Attempts:      summary.Attempts,
		RestartCount:  summary.RestartCount,
		BatchID:       batchID,
		SupervisorPID: supervisorPID,
	})
	runner.emit(RunEvent{Type: "refresh-results", Level: "info", Message: "刷新账号结果"})
}

func (runner *runner) watchSupervisor(ctx context.Context, cfg Config, batchID string, supervisorPID int) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	lastEventID := ""
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			event, eventErr := readSupervisorProgress(cfg.OutputDir)
			if eventErr == nil {
				eventID := event.EventID
				if eventID == "" {
					eventID = event.UpdatedAt + event.Type + event.Message + fmt.Sprintf("%d/%d/%d", event.Done, event.OK, event.Failed)
				}
				if eventID != lastEventID && (event.SupervisorPID == 0 || event.SupervisorPID == supervisorPID) {
					lastEventID = eventID
					runner.applyProgressEvent(event, cfg.Count, supervisorPID)
				}
			}
			status, err := readSupervisorStatus(cfg.OutputDir)
			if err != nil || status.PID != supervisorPID {
				continue
			}
			summary, err := readSupervisorSummary(cfg.OutputDir, batchID)
			if err != nil {
				continue
			}
			if eventErr != nil {
				runner.updateStatus(RunStatus{
					Running:           status.Running,
					Phase:             status.Phase,
					Message:           status.Message,
					Count:             cfg.Count,
					Done:              summary.Succeeded,
					OK:                summary.Succeeded,
					Failed:            summary.Failed,
					Attempts:          summary.Attempts,
					RestartCount:      status.RestartCount,
					RiskProbeCount:    status.RiskProbeCount,
					RiskCooldownUntil: status.RiskCooldownUntil,
					BatchID:           batchID,
					SupervisorPID:     supervisorPID,
				})
			}
			if !status.Running || summary.FinishedAt != "" {
				runner.finishFromSummary(summary, cfg.Count, batchID, supervisorPID)
				return
			}
		}
	}
}

func (runner *runner) applyProgressEvent(event ProgressEvent, count int, supervisorPID int) {
	runner.mu.Lock()
	status := runner.status
	status.Running = event.Phase != "done" && event.Phase != "failed"
	status.Phase = event.Phase
	status.Message = event.Message
	status.Count = count
	status.Done = event.Done
	status.OK = event.OK
	status.Failed = event.Failed
	status.Attempts = event.Attempts
	status.Current = event.Current
	status.RestartCount = event.RestartCount
	status.RiskProbeCount = event.RiskProbeCount
	status.RiskCooldownUntil = event.RiskCooldownUntil
	status.BatchID = event.BatchID
	if status.BatchID == "" {
		status.BatchID = "resume-200"
	}
	status.SupervisorPID = supervisorPID
	runner.status = status
	runner.mu.Unlock()
	runner.emitEvent("run-status", status)
	runner.log("info", event.Message)
	if event.Type == "account-result" && event.Result != nil {
		value := *event.Result
		runner.emit(RunEvent{Type: "result", Level: map[bool]string{true: "success", false: "error"}[value.OK], Message: resultLabel(value), Result: &value})
	}
}

func readSupervisorProgress(outputDir string) (ProgressEvent, error) {
	var event ProgressEvent
	data, err := os.ReadFile(filepath.Join(outputDir, "supervisor.progress.json"))
	if err != nil {
		return event, err
	}
	err = json.Unmarshal(data, &event)
	return event, err
}

func (runner *runner) scanOutput(group *sync.WaitGroup, stream string, reader interface{ Read([]byte) (int, error) }, cfg Config, dryRun bool) {
	defer group.Done()
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if runner.handleProgressLine(line, cfg, dryRun) {
			continue
		}
		level := "info"
		if stream == "stderr" {
			level = "warn"
		}
		if strings.Contains(strings.ToLower(line), "error") || strings.Contains(strings.ToLower(line), "failed") {
			level = "error"
		}
		runner.log(level, redactLine(line))
	}
}

func (runner *runner) handleProgressLine(line string, cfg Config, dryRun bool) bool {
	if dryRun {
		return false
	}
	if !strings.HasPrefix(line, "{") {
		return false
	}
	var event ProgressEvent
	if err := json.Unmarshal([]byte(line), &event); err != nil {
		return false
	}
	if event.Type != "progress" && event.Type != "account-result" {
		return false
	}
	runner.mu.Lock()
	status := runner.status
	status.Running = event.Phase != "done" && event.Phase != "failed"
	status.Phase = event.Phase
	status.Message = event.Message
	status.Count = cfg.Count
	status.Done = event.Done
	status.OK = event.OK
	status.Failed = event.Failed
	status.Attempts = event.Attempts
	status.Current = event.Current
	status.RestartCount = event.RestartCount
	status.RiskProbeCount = event.RiskProbeCount
	status.RiskCooldownUntil = event.RiskCooldownUntil
	status.BatchID = event.BatchID
	if event.BatchID == "" {
		status.BatchID = "resume-200"
	}
	runner.status = status
	runner.mu.Unlock()
	runner.emitEvent("run-status", status)
	if event.Type == "account-result" && event.Result != nil {
		value := *event.Result
		runner.emit(RunEvent{Type: "result", Level: map[bool]string{true: "success", false: "error"}[value.OK], Message: resultLabel(value), Result: &value})
	} else {
		runner.log("info", event.Message)
	}
	return true
}

type SupervisorSummary struct {
	BatchID      string `json:"batchId"`
	Requested    int    `json:"requested"`
	Succeeded    int    `json:"succeeded"`
	Failed       int    `json:"failed"`
	Attempts     int    `json:"attempts"`
	FinishedAt   string `json:"finishedAt"`
	RestartCount int    `json:"restartCount"`
}

func readSupervisorStatus(outputDir string) (SupervisorStatus, error) {
	var status SupervisorStatus
	data, err := os.ReadFile(filepath.Join(outputDir, "supervisor.status.json"))
	if err != nil {
		return status, err
	}
	err = json.Unmarshal(data, &status)
	return status, err
}

func readSupervisorSummary(outputDir, batchID string) (SupervisorSummary, error) {
	var summary SupervisorSummary
	data, err := os.ReadFile(filepath.Join(outputDir, batchID, "summary.json"))
	if err != nil {
		return summary, err
	}
	err = json.Unmarshal(data, &summary)
	if summary.RestartCount == 0 {
		status, statusErr := readSupervisorStatus(outputDir)
		if statusErr == nil {
			summary.RestartCount = status.RestartCount
		}
	}
	return summary, err
}

var (
	sensitiveQuery      = regexp.MustCompile(`(?i)([?&](?:k|key|token|password|secret|ticket|auth|authorization|code)=)[^&\s"'<>]+`)
	sensitiveAssignment = regexp.MustCompile(`(?i)(PANEL_KEY|IMAP_PASSWORD|ticket|Proxy-Authorization|Authorization)([=:]\s*)[^\s,;]+`)
	bearerToken         = regexp.MustCompile(`(?i)(Bearer\s+)[A-Za-z0-9._~+/=-]+`)
)

func redactLine(line string) string {
	redacted := sensitiveQuery.ReplaceAllString(line, "$1***")
	redacted = sensitiveAssignment.ReplaceAllString(redacted, "$1$2***")
	redacted = bearerToken.ReplaceAllString(redacted, "$1***")
	return redacted
}

func loadResults(root string) ([]Result, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	results := make([]Result, 0)
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "account-") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, entry.Name(), "result.json"))
		if err != nil {
			continue
		}
		var result Result
		if err := json.Unmarshal(data, &result); err == nil {
			results = append(results, result)
		}
	}
	sort.Slice(results, func(left, right int) bool {
		return results[left].RunDir < results[right].RunDir
	})
	return results, nil
}

func resultLabel(result Result) string {
	if result.OK {
		return fmt.Sprintf("%s · %s", result.Email, result.User)
	}
	return fmt.Sprintf("%s · %s", result.Email, result.Error)
}
