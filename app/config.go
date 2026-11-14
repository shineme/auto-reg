package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type Config struct {
	PanelBase          string `json:"panelBase"`
	PanelKey           string `json:"panelKey"`
	MailDomain         string `json:"mailDomain"`
	MailPrefix         string `json:"mailPrefix"`
	Mailbox            string `json:"mailbox"`
	ImapHost           string `json:"imapHost"`
	ImapPort           int    `json:"imapPort"`
	ImapUser           string `json:"imapUser"`
	ImapPassword       string `json:"imapPassword"`
	ProxyURL           string `json:"proxyUrl"`
	Count              int    `json:"count"`
	PrewarmConcurrency int    `json:"prewarmConcurrency"`
	ChromePath         string `json:"chromePath"`
	OutputDir          string `json:"outputDir"`
	ScriptsDir         string `json:"scriptsDir"`
	CDPPort            int    `json:"cdpPort"`
}

func DefaultConfig() Config {
	workspace := detectWorkspace()
	return Config{
		PanelBase:          "https://panel.example.com",
		MailDomain:         "example.com",
		MailPrefix:         "mm",
		Mailbox:            "mailbox@example.com",
		ImapHost:           "imap.example.com",
		ImapPort:           993,
		ImapUser:           "mailbox@example.com",
		ProxyURL:           "",
		Count:              1,
		PrewarmConcurrency: 4,
		ChromePath:         `C:\Program Files\Google\Chrome\Application\chrome.exe`,
		OutputDir:          filepath.Join(workspace, "artifacts", "gui-batch"),
		ScriptsDir:         filepath.Join(workspace, "scripts"),
		CDPPort:            9333,
	}
}

func detectWorkspace() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	for candidate := cwd; ; candidate = filepath.Dir(candidate) {
		if _, err := os.Stat(filepath.Join(candidate, "PROTOCOL.md")); err == nil {
			return candidate
		}
		parent := filepath.Dir(candidate)
		if parent == candidate {
			return cwd
		}
	}
}

func configPath() (string, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "minimax-registrar", "config.json"), nil
}

func LoadConfig() (Config, error) {
	cfg := DefaultConfig()
	path, err := configPath()
	if err != nil {
		return cfg, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	cfg.applyDefaults()
	return cfg, nil
}

func (cfg *Config) applyDefaults() {
	defaults := DefaultConfig()
	if cfg.PanelBase == "" {
		cfg.PanelBase = defaults.PanelBase
	}
	if cfg.MailDomain == "" {
		cfg.MailDomain = defaults.MailDomain
	}
	if cfg.MailPrefix == "" {
		cfg.MailPrefix = defaults.MailPrefix
	}
	if cfg.ImapHost == "" {
		cfg.ImapHost = defaults.ImapHost
	}
	if cfg.ImapPort == 0 {
		cfg.ImapPort = defaults.ImapPort
	}
	if cfg.Count <= 0 {
		cfg.Count = defaults.Count
	}
	if cfg.PrewarmConcurrency <= 0 {
		cfg.PrewarmConcurrency = defaults.PrewarmConcurrency
	}
	if cfg.CDPPort == 0 {
		cfg.CDPPort = defaults.CDPPort
	}
	if cfg.ChromePath == "" {
		cfg.ChromePath = defaults.ChromePath
	}
	if cfg.OutputDir == "" {
		cfg.OutputDir = defaults.OutputDir
	}
	if cfg.ScriptsDir == "" {
		cfg.ScriptsDir = defaults.ScriptsDir
	}
}

func (cfg Config) Validate() error {
	for name, value := range map[string]string{
		"面板地址":     cfg.PanelBase,
		"面板密钥":     cfg.PanelKey,
		"邮箱域名":     cfg.MailDomain,
		"IMAP 主机":  cfg.ImapHost,
		"IMAP 用户":  cfg.ImapUser,
		"IMAP 授权码": cfg.ImapPassword,
		"脚本目录":     cfg.ScriptsDir,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s不能为空", name)
		}
	}
	if cfg.Count < 1 || cfg.Count > 200 {
		return errors.New("批量数量必须在 1 到 200 之间")
	}
	if cfg.PrewarmConcurrency < 1 || cfg.PrewarmConcurrency > 16 {
		return errors.New("预热并发必须在 1 到 16 之间")
	}
	if cfg.ImapPort < 1 || cfg.ImapPort > 65535 {
		return errors.New("IMAP 端口无效")
	}
	if cfg.CDPPort < 1 || cfg.CDPPort > 65535 {
		return errors.New("CDP 端口无效")
	}
	if _, err := os.Stat(filepath.Join(cfg.ScriptsDir, "minimax-fast-batch.mjs")); err != nil {
		return fmt.Errorf("找不到 scripts/minimax-fast-batch.mjs：%w", err)
	}
	if _, err := NormalizeProxyURL(cfg.ProxyURL); err != nil {
		return err
	}
	if _, err := url.Parse(cfg.PanelBase); err != nil {
		return fmt.Errorf("面板地址无效：%w", err)
	}
	return nil
}

func (cfg Config) ValidatePanelCheck() error {
	base := strings.TrimSpace(cfg.PanelBase)
	if base == "" {
		return errors.New("面板地址不能为空")
	}
	parsed, err := url.Parse(base)
	if err != nil || (strings.ToLower(parsed.Scheme) != "http" && strings.ToLower(parsed.Scheme) != "https") || parsed.Host == "" {
		return errors.New("面板地址必须是完整的 http/https URL")
	}
	if strings.TrimSpace(cfg.PanelKey) == "" {
		return errors.New("面板密钥不能为空")
	}
	if strings.TrimSpace(cfg.ScriptsDir) == "" {
		return errors.New("脚本目录不能为空")
	}
	if _, err := os.Stat(filepath.Join(cfg.ScriptsDir, "minimax-fast-batch.mjs")); err != nil {
		return fmt.Errorf("找不到 scripts/minimax-fast-batch.mjs：%w", err)
	}
	if _, err := NormalizeProxyURL(cfg.ProxyURL); err != nil {
		return err
	}
	return nil
}

func SaveConfig(cfg Config) error {
	cfg.applyDefaults()
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

var (
	escapedColon    = regexp.MustCompile(`\\:`)
	compactProxyURL = regexp.MustCompile(`(?i)^([a-z][a-z0-9+.-]*)://([^:/@?#]+):(.+):([^:/@?#]+):(\d+)([/?#].*)?$`)
)

func expandCompactProxyURL(value string) string {
	match := compactProxyURL.FindStringSubmatch(value)
	if match == nil {
		return value
	}
	userInfo := url.UserPassword(match[2], match[3]).String()
	return fmt.Sprintf("%s://%s@%s:%s%s", match[1], userInfo, match[4], match[5], match[6])
}

func NormalizeProxyURL(raw string) (string, error) {
	value := escapedColon.ReplaceAllString(strings.TrimSpace(raw), ":")
	value = expandCompactProxyURL(value)
	if value == "" {
		return "", nil
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return "", fmt.Errorf("代理地址无效：%w", err)
	}
	switch parsed.Scheme {
	case "http", "https", "socks5", "socks5h":
	default:
		return "", errors.New("代理仅支持 http、https、socks5、socks5h")
	}
	if parsed.Hostname() == "" || parsed.Port() == "" {
		return "", errors.New("代理地址必须包含主机和端口")
	}
	return parsed.String(), nil
}
