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
	PanelBase               string   `json:"panelBase"`
	PanelKey                string   `json:"panelKey"`
	MailMode                string   `json:"mailMode"`
	MailDomain              string   `json:"mailDomain"`
	MailPrefix              string   `json:"mailPrefix"`
	Mailbox                 string   `json:"mailbox"`
	ImapHost                string   `json:"imapHost"`
	ImapPort                int      `json:"imapPort"`
	ImapUser                string   `json:"imapUser"`
	ImapPassword            string   `json:"imapPassword"`
	HotmailRaw              string   `json:"hotmailRaw"`
	HotmailPool             string   `json:"hotmailPool"`
	HotmailPreflightWorkers int      `json:"hotmailPreflightWorkers"`
	ProxyURL                string   `json:"proxyUrl"`
	ProxyDirectHosts        []string `json:"proxyDirectHosts"`
	Count                   int      `json:"count"`
	Workers                 int      `json:"workers"`
	ChromePath              string   `json:"chromePath"`
	OutputDir               string   `json:"outputDir"`
	ScriptsDir              string   `json:"scriptsDir"`
}

func DefaultConfig() Config {
	workspace := detectWorkspace()
	return Config{
		PanelBase:               "https://panel.example.com",
		MailMode:                "imap",
		MailDomain:              "example.com",
		MailPrefix:              "mm",
		Mailbox:                 "mailbox@example.com",
		ImapHost:                "imap.example.com",
		ImapPort:                993,
		ImapUser:                "mailbox@example.com",
		HotmailRaw:              "",
		HotmailPool:             "",
		HotmailPreflightWorkers: 8,
		ProxyURL:                "",
		Count:                   1,
		Workers:                 8,
		ChromePath:              `C:\Program Files\Google\Chrome\Application\chrome.exe`,
		OutputDir:               filepath.Join(workspace, "artifacts", "gui-batch"),
		ScriptsDir:              filepath.Join(workspace, "scripts"),
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
	if cfg.MailMode == "" {
		cfg.MailMode = defaults.MailMode
	}
	cfg.MailMode = strings.ToLower(strings.TrimSpace(cfg.MailMode))
	if cfg.MailMode != "hotmail" && cfg.MailMode != "hotmail-graph" {
		cfg.MailMode = "imap"
	} else {
		cfg.MailMode = "hotmail"
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
	if cfg.Workers <= 0 {
		cfg.Workers = defaults.Workers
	}
	if cfg.Workers > 8 {
		cfg.Workers = 8
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
	if cfg.HotmailPool == "" && cfg.OutputDir != "" {
		cfg.HotmailPool = filepath.Join(cfg.OutputDir, "hotmail-pool.json")
	}
	if cfg.HotmailPreflightWorkers <= 0 {
		cfg.HotmailPreflightWorkers = defaults.HotmailPreflightWorkers
	}
	if cfg.HotmailPreflightWorkers > 32 {
		cfg.HotmailPreflightWorkers = 32
	}
	defaultDirectHosts := []string{
		"localhost",
		"127.0.0.1",
		"::1",
		"web.open-traffic.com",
		"account.minimax.io",
	}
	if len(cfg.ProxyDirectHosts) == 0 {
		cfg.ProxyDirectHosts = defaultDirectHosts
	} else {
		seen := map[string]bool{}
		for _, host := range append([]string{}, defaultDirectHosts...) {
			seen[strings.ToLower(strings.TrimSpace(host))] = true
		}
		for _, host := range cfg.ProxyDirectHosts {
			clean := strings.ToLower(strings.TrimSpace(host))
			if clean != "" {
				seen[clean] = true
			}
		}
		cfg.ProxyDirectHosts = []string{}
		for host := range seen {
			cfg.ProxyDirectHosts = append(cfg.ProxyDirectHosts, host)
		}
	}
}

func (cfg Config) Validate() error {
	required := map[string]string{
		"面板地址":                       cfg.PanelBase,
		"面板密钥":                       cfg.PanelKey,
		"脚本目录":                       cfg.ScriptsDir,
		"MINIMAX_SIGNATURE_SALT":     os.Getenv("MINIMAX_SIGNATURE_SALT"),
		"MINIMAX_YY_SUFFIX":          os.Getenv("MINIMAX_YY_SUFFIX"),
		"MINIMAX_RECAPTCHA_SITE_KEY": os.Getenv("MINIMAX_RECAPTCHA_SITE_KEY"),
	}
	if cfg.MailMode == "hotmail" {
		required["Hotmail 账号池"] = cfg.HotmailPool
	} else {
		required["邮箱域名"] = cfg.MailDomain
		required["IMAP 主机"] = cfg.ImapHost
		required["IMAP 用户"] = cfg.ImapUser
		required["IMAP 授权码"] = cfg.ImapPassword
	}
	for name, value := range required {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s不能为空", name)
		}
	}
	if cfg.Count < 1 || cfg.Count > 200 {
		return errors.New("批量数量必须在 1 到 200 之间")
	}
	if cfg.MailMode != "hotmail" {
		if cfg.ImapPort < 1 || cfg.ImapPort > 65535 {
			return errors.New("IMAP 端口无效")
		}
	} else {
		if _, err := os.Stat(filepath.Join(cfg.ScriptsDir, "hotmail-graph.mjs")); err != nil {
			return fmt.Errorf("找不到 scripts/hotmail-graph.mjs：%w", err)
		}
		if strings.TrimSpace(cfg.HotmailRaw) == "" {
			if _, err := os.Stat(cfg.HotmailPool); err != nil {
				return errors.New("Hotmail 模式下请先导入 Graph 账号，或确保账号池文件已存在")
			}
		}
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
	for name, value := range map[string]string{
		"MINIMAX_SIGNATURE_SALT":     os.Getenv("MINIMAX_SIGNATURE_SALT"),
		"MINIMAX_YY_SUFFIX":          os.Getenv("MINIMAX_YY_SUFFIX"),
		"MINIMAX_RECAPTCHA_SITE_KEY": os.Getenv("MINIMAX_RECAPTCHA_SITE_KEY"),
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s不能为空", name)
		}
	}
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
