package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type PanelCheckResult struct {
	OK                bool     `json:"ok"`
	PanelBase         string   `json:"panelBase"`
	Cookies           []string `json:"cookies"`
	ProviderKeys      []string `json:"providerKeys"`
	HasProvidersArray bool     `json:"hasProvidersArray"`
	CheckedAt         string   `json:"checkedAt"`
	Message           string   `json:"message"`
}

func (app *App) TestPanel(config Config) (PanelCheckResult, error) {
	if err := config.ValidatePanelCheck(); err != nil {
		return PanelCheckResult{}, err
	}
	if _, err := exec.LookPath("node"); err != nil {
		return PanelCheckResult{}, fmt.Errorf("未找到 Node.js：%w", err)
	}
	localProxy, err := app.bridge.Start(config.ProxyURL)
	if err != nil {
		return PanelCheckResult{}, err
	}
	env := append(os.Environ(),
		"MINIMAX_SIGNATURE_SALT="+os.Getenv("MINIMAX_SIGNATURE_SALT"),
		"MINIMAX_YY_SUFFIX="+os.Getenv("MINIMAX_YY_SUFFIX"),
		"MINIMAX_RECAPTCHA_SITE_KEY="+os.Getenv("MINIMAX_RECAPTCHA_SITE_KEY"),
		"PANEL_BASE="+config.PanelBase,
		"PANEL_KEY="+config.PanelKey,
		"PANEL_CHECK_ONLY=1",
		"NODE_NO_WARNINGS=1",
	)
	env = append(env, proxyEnv(localProxy)...)
	command := exec.Command("node", filepath.Join(config.ScriptsDir, "minimax-fast-batch.mjs"))
	command.Env = env
	command.Dir = detectWorkspace()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	command.WaitDelay = time.Second
	if err := command.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = strings.TrimSpace(stdout.String())
		}
		return PanelCheckResult{}, fmt.Errorf("面板检查失败：%s：%w", redactLine(message), err)
	}
	var result PanelCheckResult
	decoder := json.NewDecoder(strings.NewReader(strings.TrimSpace(stdout.String())))
	if err := decoder.Decode(&result); err != nil {
		return PanelCheckResult{}, fmt.Errorf("面板检查结果无效：%w", err)
	}
	if !result.OK {
		return PanelCheckResult{}, fmt.Errorf("面板检查未通过：%s", result.Message)
	}
	result.Message = fmt.Sprintf("面板连接正常，持有会话 Cookie，读取到 %d 个供应商字段", len(result.ProviderKeys))
	app.runner.log("info", result.Message)
	return result, nil
}
