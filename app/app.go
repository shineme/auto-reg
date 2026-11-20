package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx    context.Context
	bridge *proxyBridge
	runner *runner
	job    *processJob
}

func NewApp() *App {
	job, err := newProcessJob()
	if err != nil {
		panic(err)
	}
	app := &App{bridge: &proxyBridge{}, job: job}
	app.runner = newRunner(app)
	return app
}

func (app *App) startup(ctx context.Context) {
	app.ctx = ctx
	cfg, err := LoadConfig()
	if err == nil {
		status, statusErr := readSupervisorStatus(cfg.OutputDir)
		if statusErr == nil && status.Running && processAlive(status.PID) {
			_ = app.runner.Start(cfg, false)
		} else if statusErr == nil && !status.Running && (status.Phase == "domain-blocked" || status.Phase == "environment-unavailable") {
			if _, err := clearRunState(cfg.OutputDir, false); err == nil {
				app.runner.setStatus(RunStatus{Running: false, Phase: "idle", Message: "旧风控状态已清理，可以重新开始", Count: cfg.Count})
			}
		}
	}
	if os.Getenv("MINIMAX_AUTOSTART") == "1" {
		if err == nil {
			_ = app.runner.Start(cfg, os.Getenv("MINIMAX_AUTOSTART_DRY_RUN") == "1")
		}
	}
}

func (app *App) shutdown(ctx context.Context) {
	app.runner.Stop()
	app.bridge.Stop()
	app.job.Stop()
}

func (app *App) GetConfig() (Config, error) {
	return LoadConfig()
}

func (app *App) SaveConfig(config Config) error {
	return SaveConfig(config)
}

func (app *App) ValidateConfig(config Config) error {
	return config.Validate()
}

func (app *App) NormalizeProxy(raw string) (string, error) {
	return NormalizeProxyURL(raw)
}

func (app *App) StartRun(config Config, dryRun bool) error {
	if err := SaveConfig(config); err != nil {
		return err
	}
	return app.runner.Start(config, dryRun)
}

func (app *App) StopRun() {
	app.runner.Stop()
}

func (app *App) ClearRunState(includeHistory bool) (map[string]interface{}, error) {
	if app.runner.Running() || app.runner.HasActiveSupervisor() {
		return nil, errors.New("请先停止当前任务，再清理运行记录")
	}
	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}
	result, err := clearRunState(cfg.OutputDir, includeHistory)
	if err != nil {
		return nil, err
	}
	app.runner.setStatus(RunStatus{Running: false, Phase: "idle", Message: "旧运行状态已清理，可以重新开始", Count: cfg.Count})
	return result, nil
}

func clearRunState(outputDir string, includeHistory bool) (map[string]interface{}, error) {
	removed := make([]string, 0)
	paths := []string{
		filepath.Join(outputDir, "risk-state.json"),
		filepath.Join(outputDir, "supervisor.status.json"),
		filepath.Join(outputDir, "supervisor.progress.json"),
		filepath.Join(outputDir, "supervisor.lock"),
	}
	entries, err := os.ReadDir(outputDir)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			paths = append(paths, filepath.Join(outputDir, entry.Name(), "risk-control.json"))
			if includeHistory {
				paths = append(paths,
					filepath.Join(outputDir, entry.Name(), "summary.json"),
					filepath.Join(outputDir, entry.Name(), "dryrun.json"),
					filepath.Join(outputDir, entry.Name(), "dryrun-session.json"),
				)
				accountEntries, accountErr := os.ReadDir(filepath.Join(outputDir, entry.Name()))
				if accountErr != nil {
					continue
				}
				for _, accountEntry := range accountEntries {
					if accountEntry.IsDir() && strings.HasPrefix(accountEntry.Name(), "account-") {
						paths = append(paths, filepath.Join(outputDir, entry.Name(), accountEntry.Name()))
					}
				}
			}
		}
	}
	if includeHistory {
		paths = append(paths, filepath.Join(outputDir, "resume-ledger.json"))
	}
	for _, stateFile := range paths {
		if err := os.RemoveAll(stateFile); err != nil {
			return nil, err
		}
		if _, err := os.Lstat(stateFile); errors.Is(err, os.ErrNotExist) {
			removed = append(removed, stateFile)
		}
	}
	return map[string]interface{}{
		"ok":             true,
		"includeHistory": includeHistory,
		"cleared":        removed,
		"outputDir":      outputDir,
	}, nil
}

func (app *App) GetRunStatus() RunStatus {
	return app.runner.Status()
}

func (app *App) GetRunEvents() []RunEvent {
	return app.runner.Events()
}

func (app *App) GetLatestResults() ([]Result, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}
	resumePath := filepath.Join(cfg.OutputDir, "resume-200")
	if _, err := os.Stat(filepath.Join(resumePath, "summary.json")); err == nil {
		return loadResults(resumePath)
	}
	entries, err := os.ReadDir(cfg.OutputDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Result{}, nil
		}
		return nil, err
	}
	var latest string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(cfg.OutputDir, entry.Name(), "summary.json")); err == nil && entry.Name() > latest {
			latest = entry.Name()
		}
	}
	if latest == "" {
		return []Result{}, nil
	}
	return loadResults(filepath.Join(cfg.OutputDir, latest))
}

func (app *App) OpenOutputDir() error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(cfg.OutputDir, 0o755); err != nil {
		return err
	}
	return openPath(cfg.OutputDir)
}

func (app *App) CheckRuntime() (map[string]string, error) {
	results := map[string]string{}
	if _, err := exec.LookPath("node"); err == nil {
		output, _ := exec.Command("node", "--version").Output()
		results["node"] = strings.TrimSpace(string(output))
	} else {
		results["node"] = "missing"
	}
	cfg, _ := LoadConfig()
	if _, err := os.Stat(cfg.ChromePath); err == nil {
		results["chrome"] = "ok"
	} else {
		results["chrome"] = "missing"
	}
	if _, err := os.Stat(filepath.Join(cfg.ScriptsDir, "minimax-fast-batch.mjs")); err == nil {
		results["scripts"] = "ok"
	} else {
		results["scripts"] = "missing"
	}
	data, _ := json.Marshal(results)
	if app.ctx != nil {
		runtime.EventsEmit(app.ctx, "runtime-check", string(data))
	}
	return results, nil
}

type HotmailPoolStatus struct {
	OK        bool   `json:"ok"`
	FilePath  string `json:"filePath"`
	Total     int    `json:"total"`
	Available int    `json:"available"`
	Reserved  int    `json:"reserved"`
	Used      int    `json:"used"`
	Error     int    `json:"error"`
	UpdatedAt string `json:"updatedAt"`
	Message   string `json:"message"`
}

type HotmailImportResult struct {
	OK        bool                     `json:"ok"`
	FilePath  string                   `json:"filePath"`
	Imported  int                      `json:"imported"`
	Updated   int                      `json:"updated"`
	Total     int                      `json:"total"`
	Available int                      `json:"available"`
	Used      int                      `json:"used"`
	Errors    []map[string]interface{} `json:"errors"`
	Message   string                   `json:"message"`
}

type HotmailTestResult struct {
	OK            bool   `json:"ok"`
	Email         string `json:"email"`
	Scope         string `json:"scope"`
	MessageCount  int    `json:"messageCount"`
	LatestSubject string `json:"latestSubject"`
	CheckedAt     string `json:"checkedAt"`
	Message       string `json:"message"`
}

type HotmailPreflightResult struct {
	OK               bool                     `json:"ok"`
	FilePath         string                   `json:"filePath"`
	ReleasedReserved int                      `json:"releasedReserved"`
	RecycledSoft     int                      `json:"recycledSoftErrors"`
	Checked          int                      `json:"checked"`
	Workers          int                      `json:"workers"`
	OkCount          int                      `json:"okCount"`
	ErrorCount       int                      `json:"errorCount"`
	Available        int                      `json:"available"`
	Reserved         int                      `json:"reserved"`
	Used             int                      `json:"used"`
	Error            int                      `json:"error"`
	Total            int                      `json:"total"`
	Results          []map[string]interface{} `json:"results"`
	Message          string                   `json:"message"`
}

type HotmailPoolCleanupResult struct {
	OK            bool     `json:"ok"`
	FilePath      string   `json:"filePath"`
	Removed       int      `json:"removed"`
	RemovedEmails []string `json:"removedEmails"`
	Before        int      `json:"before"`
	Total         int      `json:"total"`
	Available     int      `json:"available"`
	Reserved      int      `json:"reserved"`
	Used          int      `json:"used"`
	Error         int      `json:"error"`
	Message       string   `json:"message"`
}

func (app *App) ImportHotmailAccounts(config Config, replace bool) (HotmailImportResult, error) {
	config.applyDefaults()
	if strings.TrimSpace(config.HotmailRaw) == "" {
		return HotmailImportResult{}, errors.New("请先粘贴 Hotmail Graph 账号文本")
	}
	if err := SaveConfig(config); err != nil {
		return HotmailImportResult{}, err
	}
	env := []string{
		"HOTMAIL_COMMAND=import",
		"HOTMAIL_POOL_PATH=" + config.HotmailPool,
		"HOTMAIL_RAW=" + config.HotmailRaw,
		"BATCH_ROOT=" + config.OutputDir,
		"NODE_NO_WARNINGS=1",
	}
	if replace {
		env = append(env, "HOTMAIL_REPLACE=1")
	}
	var result HotmailImportResult
	if err := runHotmailScript(config, env, &result); err != nil {
		return HotmailImportResult{}, err
	}
	result.Message = fmt.Sprintf("已导入 Hotmail 账号：新增 %d，更新 %d，可用 %d / 总计 %d", result.Imported, result.Updated, result.Available, result.Total)
	app.runner.log("info", result.Message)
	return result, nil
}

func (app *App) GetHotmailPoolStatus(config Config) (HotmailPoolStatus, error) {
	config.applyDefaults()
	env := []string{
		"HOTMAIL_COMMAND=status",
		"HOTMAIL_POOL_PATH=" + config.HotmailPool,
		"BATCH_ROOT=" + config.OutputDir,
		"NODE_NO_WARNINGS=1",
	}
	var result HotmailPoolStatus
	if err := runHotmailScript(config, env, &result); err != nil {
		return HotmailPoolStatus{}, err
	}
	result.Message = fmt.Sprintf("Hotmail 账号池：可用 %d · 失效 %d · 已用 %d · 总计 %d", result.Available, result.Error, result.Used, result.Total)
	return result, nil
}

func (app *App) TestHotmailAccounts(config Config, limit int) (HotmailTestResult, error) {
	config.applyDefaults()
	if limit <= 0 {
		limit = 1
	}
	if err := SaveConfig(config); err != nil {
		return HotmailTestResult{}, err
	}
	if strings.TrimSpace(config.HotmailRaw) != "" {
		if _, err := app.ImportHotmailAccounts(config, false); err != nil {
			return HotmailTestResult{}, err
		}
	}
	status, err := app.GetHotmailPoolStatus(config)
	if err != nil {
		return HotmailTestResult{}, err
	}
	if status.Available < 1 && status.Total < 1 {
		return HotmailTestResult{}, errors.New("Hotmail 账号池为空，请先导入 Graph 账号")
	}
	poolData, err := os.ReadFile(config.HotmailPool)
	if err != nil {
		return HotmailTestResult{}, err
	}
	var pool struct {
		Accounts []map[string]interface{} `json:"accounts"`
	}
	if err := json.Unmarshal(poolData, &pool); err != nil {
		return HotmailTestResult{}, err
	}
	var target map[string]interface{}
	for _, account := range pool.Accounts {
		statusValue, _ := account["status"].(string)
		if statusValue == "available" || statusValue == "reserved" || statusValue == "" {
			target = account
			break
		}
	}
	if target == nil && len(pool.Accounts) > 0 {
		target = pool.Accounts[0]
	}
	if target == nil {
		return HotmailTestResult{}, errors.New("没有可测试的 Hotmail 账号")
	}
	accountJSON, err := json.Marshal(target)
	if err != nil {
		return HotmailTestResult{}, err
	}
	env := []string{
		"HOTMAIL_COMMAND=test",
		"HOTMAIL_POOL_PATH=" + config.HotmailPool,
		"HOTMAIL_ACCOUNT_JSON=" + string(accountJSON),
		"BATCH_ROOT=" + config.OutputDir,
		"NODE_NO_WARNINGS=1",
	}
	var result HotmailTestResult
	if err := runHotmailScript(config, env, &result); err != nil {
		return HotmailTestResult{}, err
	}
	_ = limit
	result.Message = fmt.Sprintf("Hotmail Graph 可用：%s，最近邮件 %d 封", result.Email, result.MessageCount)
	app.runner.log("info", result.Message)
	return result, nil
}

func (app *App) PreflightHotmailAccounts(config Config, limit int) (HotmailPreflightResult, error) {
	config.applyDefaults()
	if err := SaveConfig(config); err != nil {
		return HotmailPreflightResult{}, err
	}
	if strings.TrimSpace(config.HotmailRaw) != "" {
		if _, err := app.ImportHotmailAccounts(config, false); err != nil {
			return HotmailPreflightResult{}, err
		}
	}
	app.runner.log("info", "开始批量测活 Hotmail 账号池")
	env := []string{
		"HOTMAIL_COMMAND=preflight",
		"HOTMAIL_POOL_PATH=" + config.HotmailPool,
		"HOTMAIL_RELEASE_RESERVED=1",
		fmt.Sprintf("HOTMAIL_PREFLIGHT_CONCURRENCY=%d", config.HotmailPreflightWorkers),
		"BATCH_ROOT=" + config.OutputDir,
		"NODE_NO_WARNINGS=1",
	}
	if limit > 0 {
		env = append(env, fmt.Sprintf("HOTMAIL_PREFLIGHT_LIMIT=%d", limit))
	}
	var result HotmailPreflightResult
	if err := runHotmailScriptWithProgress(app, config, env, &result); err != nil {
		return HotmailPreflightResult{}, err
	}
	result.Message = fmt.Sprintf("测活完成：可用 %d · 失效 %d · 已检查 %d · 总计 %d", result.Available, result.Error, result.Checked, result.Total)
	app.runner.log("info", result.Message)
	return result, nil
}

func (app *App) PurgeInvalidHotmailAccounts(config Config) (HotmailPoolCleanupResult, error) {
	config.applyDefaults()
	if err := SaveConfig(config); err != nil {
		return HotmailPoolCleanupResult{}, err
	}
	env := []string{
		"HOTMAIL_COMMAND=purge-invalid",
		"HOTMAIL_POOL_PATH=" + config.HotmailPool,
		"BATCH_ROOT=" + config.OutputDir,
		"NODE_NO_WARNINGS=1",
	}
	var result HotmailPoolCleanupResult
	if err := runHotmailScript(config, env, &result); err != nil {
		return HotmailPoolCleanupResult{}, err
	}
	result.Message = fmt.Sprintf("已移除失效账号 %d 个，剩余可用 %d / 总计 %d", result.Removed, result.Available, result.Total)
	app.runner.log("info", result.Message)
	return result, nil
}

func (app *App) KeepValidHotmailAccounts(config Config) (HotmailPoolCleanupResult, error) {
	config.applyDefaults()
	if err := SaveConfig(config); err != nil {
		return HotmailPoolCleanupResult{}, err
	}
	env := []string{
		"HOTMAIL_COMMAND=keep-valid",
		"HOTMAIL_POOL_PATH=" + config.HotmailPool,
		"BATCH_ROOT=" + config.OutputDir,
		"NODE_NO_WARNINGS=1",
	}
	var result HotmailPoolCleanupResult
	if err := runHotmailScript(config, env, &result); err != nil {
		return HotmailPoolCleanupResult{}, err
	}
	result.Message = fmt.Sprintf("已仅保留有效账号：可用 %d · 已用 %d · 总计 %d（移除 %d）", result.Available, result.Used, result.Total, result.Removed)
	app.runner.log("info", result.Message)
	return result, nil
}

func runHotmailScript(config Config, extraEnv []string, output any) error {
	return runHotmailScriptWithProgress(nil, config, extraEnv, output)
}

func runHotmailScriptWithProgress(app *App, config Config, extraEnv []string, output any) error {
	if _, err := exec.LookPath("node"); err != nil {
		return fmt.Errorf("未找到 Node.js：%w", err)
	}
	script := filepath.Join(config.ScriptsDir, "hotmail-graph.mjs")
	if _, err := os.Stat(script); err != nil {
		return fmt.Errorf("找不到 scripts/hotmail-graph.mjs：%w", err)
	}
	command := exec.Command("node", script)
	command.Dir = detectWorkspace()
	command.Env = append(os.Environ(), extraEnv...)
	stdout, err := command.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := command.StderrPipe()
	if err != nil {
		return err
	}
	if err := command.Start(); err != nil {
		return err
	}
	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer
	done := make(chan struct{}, 2)
	go func() {
		defer func() { done <- struct{}{} }()
		scanner := bufio.NewScanner(io.TeeReader(stderr, &stderrBuf))
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || app == nil || app.ctx == nil {
				continue
			}
			var payload map[string]interface{}
			if err := json.Unmarshal([]byte(line), &payload); err != nil {
				continue
			}
			if payload["type"] != "hotmail-progress" {
				continue
			}
			message, _ := payload["message"].(string)
			if message != "" {
				app.runner.log("info", message)
			}
			runtime.EventsEmit(app.ctx, "hotmail-progress", payload)
		}
	}()
	go func() {
		defer func() { done <- struct{}{} }()
		_, _ = io.Copy(&stdoutBuf, stdout)
	}()
	<-done
	<-done
	if err := command.Wait(); err != nil {
		message := strings.TrimSpace(stderrBuf.String())
		if message == "" {
			message = strings.TrimSpace(stdoutBuf.String())
		}
		return fmt.Errorf("Hotmail Graph 调用失败：%s", redactLine(message))
	}
	if err := json.Unmarshal(bytes.TrimSpace(stdoutBuf.Bytes()), output); err != nil {
		return fmt.Errorf("Hotmail Graph 返回无效 JSON：%w", err)
	}
	return nil
}
func openPath(path string) error {
	command := exec.Command("explorer.exe", filepath.FromSlash(path))
	if err := command.Start(); err != nil {
		return fmt.Errorf("打开目录失败：%w", err)
	}
	return nil
}
