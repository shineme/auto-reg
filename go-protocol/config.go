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
	Panel2Base              string   `json:"panel2Base"`
	Panel2Key               string   `json:"panel2Key"`
	MailMode                string   `json:"mailMode"`
	MailDomain              string   `json:"mailDomain"`
	MailPrefix              string   `json:"mailPrefix"`
	ImapHost                string   `json:"imapHost"`
	ImapPort                int      `json:"imapPort"`
	ImapUser                string   `json:"imapUser"`
	ImapPassword            string   `json:"imapPassword"`
	HotmailRaw              string   `json:"hotmailRaw"`
	HotmailPool             string   `json:"hotmailPool"`
	HotmailPreflightWorkers int      `json:"hotmailPreflightWorkers"`
	ProxyURL                string   `json:"proxyUrl"`
	ProxyDirectHosts        []string `json:"proxyDirectHosts"`
	ProxyRegionRotate       string   `json:"proxyRegionRotate"`
	ProxySessTime           int      `json:"proxySessTime"`
	// ProxyStyle: auto / 711 / clipproxy —— 决定 sticky 会话参数的拼装方式。
	ProxyStyle string `json:"proxyStyle"`
	// ProxyState: Cliproxy 的州省参数（st-Louisiana 等），留空=不限。
	ProxyState string `json:"proxyState"`
	// ProxySticky: nil 或 true=sticky 会话；false=Rotating（每请求换 IP）。
	ProxySticky *bool `json:"proxySticky,omitempty"`
	// ProxySubscriptionURL: 订阅地址（Clash YAML / base64 链接列表均可）。
	ProxySubscriptionURL string `json:"proxySubscriptionUrl"`
	// ProxyPoolFile: 节点池本地文件路径，留空=输出目录/proxy-pool.json。
	ProxyPoolFile string `json:"proxyPoolFile"`
	// ProxyPoolUse: 开启后用节点池的存活节点替代固定代理地址。
	ProxyPoolUse bool `json:"proxyPoolUse"`
	// ProxyPoolWorkers: 节点测活并发数。
	ProxyPoolWorkers int `json:"proxyPoolWorkers"`
	// ProxyPoolCheckURL: 测活用的出口 IP 回显地址。
	ProxyPoolCheckURL string `json:"proxyPoolCheckUrl"`
	// ProxyLocalPorts: 本地代理端口池（如 sing-box 的 18001-18165），每个端口一个出口 IP。
	ProxyLocalPorts string `json:"proxyLocalPorts"`
	TicketMode              string   `json:"ticketMode"` // "cdp" or "external"
	ExternalTicketURL       string   `json:"externalTicketUrl"`
	Count                   int      `json:"count"`
	Workers                 int      `json:"workers"`
	CaptchaDirect           bool     `json:"captchaDirect"`
	MailViaProxy            bool     `json:"mailViaProxy"`
	SigninAfterRegister     bool     `json:"signinAfterRegister"`
	TimezoneID              string   `json:"timezoneId"`
	ChromePath              string   `json:"chromePath"`
	OutputDir               string   `json:"outputDir"`
	SignatureSalt           string   `json:"signatureSalt"`
	YySuffix                string   `json:"yySuffix"`
	RecaptchaSiteKey        string   `json:"recaptchaSiteKey"`
	// FixedPassword 留空=随机强密码；填了则所有账号强制用该固定密码（用于排查 1200058 强度/残留问题）。
	FixedPassword           string   `json:"fixedPassword"`
}

func DefaultConfig() Config {
	workspace := detectWorkspace()
	return Config{
		PanelBase:               "https://web.open-traffic.com",
		MailMode:                "hotmail",
		MailDomain:              "iffjewelry.com",
		MailPrefix:              "mm",
		ImapHost:                "imap.yeah.net",
		ImapPort:                993,
		ImapUser:                "clover52@yeah.net",
		HotmailPool:             filepath.Join(workspace, "artifacts", "go-protocol", "hotmail-pool.json"),
		HotmailPreflightWorkers: 8,
		ProxyURL:                "",
		ProxyDirectHosts:        []string{"localhost", "127.0.0.1", "::1", "web.open-traffic.com"},
		TicketMode:              "cdp",
		ExternalTicketURL:       "",
		Count:                   200,
		Workers:                 1,
		CaptchaDirect:           false,
		SigninAfterRegister:     true,
		TimezoneID:              "Asia/Shanghai",
		ChromePath:              `C:\Program Files\Google\Chrome\Application\chrome.exe`,
		OutputDir:               filepath.Join(workspace, "artifacts", "go-protocol"),
		SignatureSalt:           os.Getenv("MINIMAX_SIGNATURE_SALT"),
		YySuffix:                os.Getenv("MINIMAX_YY_SUFFIX"),
		RecaptchaSiteKey:        os.Getenv("MINIMAX_RECAPTCHA_SITE_KEY"),
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
	return filepath.Join(root, "minimax-go-protocol", "config.json"), nil
}

func LoadConfig() (Config, error) {
	cfg := DefaultConfig()
	path, err := configPath()
	if err != nil {
		return cfg, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		cfg.applyDefaults()
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

func (cfg *Config) applyDefaults() {
	d := DefaultConfig()
	if strings.TrimSpace(cfg.PanelBase) == "" {
		cfg.PanelBase = d.PanelBase
	}
	cfg.MailMode = strings.ToLower(strings.TrimSpace(cfg.MailMode))
	if cfg.MailMode != "hotmail" && cfg.MailMode != "hotmail-graph" {
		cfg.MailMode = "imap"
	} else {
		cfg.MailMode = "hotmail"
	}
	if strings.TrimSpace(cfg.MailDomain) == "" {
		cfg.MailDomain = d.MailDomain
	}
	if strings.TrimSpace(cfg.MailPrefix) == "" {
		cfg.MailPrefix = d.MailPrefix
	}
	if strings.TrimSpace(cfg.ImapHost) == "" {
		cfg.ImapHost = d.ImapHost
	}
	if cfg.ImapPort <= 0 {
		cfg.ImapPort = d.ImapPort
	}
	if strings.TrimSpace(cfg.OutputDir) == "" {
		cfg.OutputDir = d.OutputDir
	}
	if strings.TrimSpace(cfg.HotmailPool) == "" {
		cfg.HotmailPool = filepath.Join(cfg.OutputDir, "hotmail-pool.json")
	}
	if cfg.HotmailPreflightWorkers <= 0 {
		cfg.HotmailPreflightWorkers = d.HotmailPreflightWorkers
	}
	if cfg.HotmailPreflightWorkers > 32 {
		cfg.HotmailPreflightWorkers = 32
	}
	if cfg.Count <= 0 {
		cfg.Count = d.Count
	}
	if cfg.Workers <= 0 {
		cfg.Workers = d.Workers
	}
	if cfg.Workers > 16 {
		cfg.Workers = 16
	}
	if strings.TrimSpace(cfg.ChromePath) == "" {
		cfg.ChromePath = d.ChromePath
	}
	if strings.TrimSpace(cfg.TimezoneID) == "" {
		cfg.TimezoneID = d.TimezoneID
	}
	if strings.TrimSpace(cfg.FixedPassword) == "" {
		cfg.FixedPassword = os.Getenv("MINIMAX_FIXED_PASSWORD")
	}
	cfg.TicketMode = strings.ToLower(strings.TrimSpace(cfg.TicketMode))
	if cfg.TicketMode != "external" {
		cfg.TicketMode = "cdp"
	}
	if strings.TrimSpace(cfg.SignatureSalt) == "" {
		cfg.SignatureSalt = os.Getenv("MINIMAX_SIGNATURE_SALT")
	}
	if strings.TrimSpace(cfg.YySuffix) == "" {
		cfg.YySuffix = os.Getenv("MINIMAX_YY_SUFFIX")
	}
	if strings.TrimSpace(cfg.RecaptchaSiteKey) == "" {
		cfg.RecaptchaSiteKey = os.Getenv("MINIMAX_RECAPTCHA_SITE_KEY")
	}
	cfg.ProxyPoolFile = strings.TrimSpace(cfg.ProxyPoolFile)
	cfg.ProxySubscriptionURL = strings.TrimSpace(cfg.ProxySubscriptionURL)
	if cfg.ProxyPoolWorkers <= 0 {
		cfg.ProxyPoolWorkers = 16
	}
	if cfg.ProxyPoolWorkers > 64 {
		cfg.ProxyPoolWorkers = 64
	}
	if strings.TrimSpace(cfg.ProxyPoolCheckURL) == "" {
		cfg.ProxyPoolCheckURL = "https://api.ipify.org?format=json"
	}
	if cfg.ProxyDirectHosts == nil {
		cfg.ProxyDirectHosts = d.ProxyDirectHosts
	}
}

func (cfg Config) Validate() error {
	if strings.TrimSpace(cfg.PanelBase) == "" {
		return errors.New("面板地址不能为空")
	}
	if _, err := url.Parse(cfg.PanelBase); err != nil {
		return fmt.Errorf("面板地址无效: %w", err)
	}
	if strings.TrimSpace(cfg.PanelKey) == "" {
		return errors.New("面板密钥不能为空")
	}
	if strings.TrimSpace(cfg.SignatureSalt) == "" {
		return errors.New("MINIMAX_SIGNATURE_SALT 不能为空")
	}
	if strings.TrimSpace(cfg.YySuffix) == "" {
		return errors.New("MINIMAX_YY_SUFFIX 不能为空")
	}
	if cfg.TicketMode == "cdp" && strings.TrimSpace(cfg.RecaptchaSiteKey) == "" {
		return errors.New("MINIMAX_RECAPTCHA_SITE_KEY 不能为空")
	}
	if cfg.TicketMode == "external" && strings.TrimSpace(cfg.ExternalTicketURL) == "" {
		return errors.New("外部票据接口 URL 不能为空")
	}
	if cfg.MailMode == "imap" {
		if strings.TrimSpace(cfg.MailDomain) == "" || strings.TrimSpace(cfg.ImapHost) == "" || strings.TrimSpace(cfg.ImapUser) == "" || strings.TrimSpace(cfg.ImapPassword) == "" {
			return errors.New("IMAP 模式下域名、主机、用户名、授权码均不能为空")
		}
	}
	if _, err := NormalizeProxyURL(cfg.ProxyURL); err != nil {
		return err
	}
	if cfg.Count < 1 || cfg.Count > 5000 {
		return errors.New("目标数量必须在 1 到 5000 之间")
	}
	return nil
}

// parseProxyStyle 把配置里的风格名解析为枚举。
func parseProxyStyle(name string) ProxyStyle {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "711":
		return Style711
	case "clipproxy":
		return StyleClipProxy
	default:
		return StyleAuto
	}
}

// StickyEnabled 返回是否启用 sticky 会话（默认启用）。
func (cfg Config) StickyEnabled() bool {
	if cfg.ProxySticky == nil {
		return true
	}
	return *cfg.ProxySticky
}

// splitRegions 解析逗号分隔的区域列表（如 "JP,US,GB"）。
func splitRegions(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.ToUpper(strings.TrimSpace(p))
		if p != "" {
			out = append(out, p)
		}
	}
	return out
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
		return "", fmt.Errorf("代理地址无效: %w", err)
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https", "socks5", "socks5h":
	default:
		return "", errors.New("代理仅支持 http、https、socks5、socks5h")
	}
	if parsed.Hostname() == "" || parsed.Port() == "" {
		return "", errors.New("代理地址必须包含主机和端口")
	}
	return parsed.String(), nil
}
