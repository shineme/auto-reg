package main

import (
	"context"
	"encoding/json"
	"fmt"
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
		if status, statusErr := readSupervisorStatus(cfg.OutputDir); statusErr == nil && status.Running && processAlive(status.PID) {
			_ = app.runner.Start(cfg, false)
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

func openPath(path string) error {
	command := exec.Command("explorer.exe", filepath.FromSlash(path))
	if err := command.Start(); err != nil {
		return fmt.Errorf("打开目录失败：%w", err)
	}
	return nil
}
