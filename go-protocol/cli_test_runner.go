package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"
)

type checkinCredential struct {
	Email        string `json:"email"`
	UserID       string `json:"userID"`
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	Points       int    `json:"points"`
	Status       string `json:"status"`
	UpdatedAt    string `json:"updatedAt"`
}

// runCLICheckinAll 读取已保存的账号凭据做补签：先试当前 token，401/失效再用 refresh token 换新。
func runCLICheckinAll() {
	cfg := loadCLIConfig()
	cfg.applyDefaults()
	claimed, already, failed, points := checkinAllAccounts(cfg, func(line string) {
		fmt.Println(line)
	})
	fmt.Printf("CHECKIN_DONE 新领取=%d 今日已领=%d 失败=%d 本轮积分=%d\n", claimed, already, failed, points)
}

// checkinAllAccounts 遍历 <outputDir>/checkin 下的凭据逐个补签，返回统计。
func checkinAllAccounts(cfg Config, onLog func(string)) (claimed, already, failed, totalPoints int) {
	dir := filepath.Join(cfg.OutputDir, "checkin")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if onLog != nil {
			onLog("读取签到凭据目录失败: " + err.Error())
		}
		return 0, 0, 0, 0
	}
	client := NewMiniMaxClient(nil, "", cfg.SignatureSalt, cfg.YySuffix)
	client.SetTimezone(cfg.TimezoneID)
	ctx := context.Background()
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			failed++
			continue
		}
		var cred checkinCredential
		if err := json.Unmarshal(raw, &cred); err != nil {
			failed++
			continue
		}
		access, refresh := cred.AccessToken, cred.RefreshToken
		if refresh != "" {
			if na, nr, rerr := client.RefreshAccessToken(ctx, refresh); rerr == nil {
				access, refresh = na, nr
			}
		}
		res, err := client.CheckinWithToken(ctx, access, cred.UserID)
		if err != nil {
			failed++
			if onLog != nil {
				onLog(fmt.Sprintf("[checkin] %s 失败: %v", cred.Email, err))
			}
			continue
		}
		if res.Claimed {
			claimed++
		} else {
			already++
		}
		totalPoints += res.Points
		cred.AccessToken, cred.RefreshToken = access, refresh
		cred.Points = res.Points
		cred.UpdatedAt = time.Now().Format(time.RFC3339)
		if out, err := json.MarshalIndent(cred, "", "  "); err == nil {
			_ = os.WriteFile(path, out, 0o600)
		}
		if onLog != nil {
			onLog(fmt.Sprintf("[checkin] %s 今天已领取=%v 积分=%d", cred.Email, res.Claimed, res.Points))
		}
		time.Sleep(300 * time.Millisecond)
	}
	return claimed, already, failed, totalPoints
}

func runCLIImport(path string) {
	cfg := loadCLIConfig()
	cfg.applyDefaults()
	raw, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("读取账号文件失败: %v", err)
	}
	stats, added, err := ImportHotmailRaw(cfg.HotmailPool, string(raw))
	if err != nil {
		log.Fatalf("导入失败: %v", err)
	}
	fmt.Printf("IMPORT_DONE added=%d total=%d available=%d used=%d skipped=%d\n",
		added, stats.Total, stats.Available, stats.Used, stats.Skipped)
}

func runCLIPreflight(workers int) {
	cfg := loadCLIConfig()
	if workers > 0 {
		cfg.HotmailPreflightWorkers = workers
	}
	cfg.applyDefaults()
	var dialer *ProxyDialer
	if cfg.MailViaProxy {
		d, err := NewProxyDialer(cfg.ProxyURL)
		if err != nil {
			log.Fatalf("代理配置错误: %v", err)
		}
		dialer = d
	}
	stats, err := PreflightHotmailPool(context.Background(), cfg.HotmailPool, cfg.HotmailPreflightWorkers, dialer, func(p PreflightProgress) {
		if p.Checked%100 == 0 || p.Checked == p.Total {
			fmt.Printf("[preflight] %d/%d 可用=%d 失效=%d\n", p.Checked, p.Total, p.Available, p.Skipped)
		}
	})
	if err != nil {
		log.Fatalf("测活失败: %v", err)
	}
	fmt.Printf("PREFLIGHT_DONE total=%d available=%d skipped=%d used=%d\n", stats.Total, stats.Available, stats.Skipped, stats.Used)
}

func runCLIMailScan(workers int) {
	runCLIMailScanFull(workers, 0)
}

func runCLIMailScanFull(workers, limit int) {
	cfg := loadCLIConfig()
	cfg.applyDefaults()
	if workers <= 0 {
		workers = cfg.HotmailPreflightWorkers
	}
	report, err := ScanHotmailInboxes(context.Background(), cfg.HotmailPool, workers, limit, cfg.MailViaProxy, cfg.ProxyURL, func(line string) {
		fmt.Println(line)
	})
	if err != nil {
		log.Fatalf("收信扫描失败: %v", err)
	}
	PrintMailScanReport(report)
}

func loadCLIConfig() Config {
	cfg, _ := LoadConfig()
	if cfg.PanelKey == "" {
		if root, err := os.UserConfigDir(); err == nil {
			legacyPath := filepath.Join(root, "minimax-registrar", "config.json")
			if raw, err := os.ReadFile(legacyPath); err == nil {
				var legacy Config
				if json.Unmarshal(raw, &legacy) == nil {
					if cfg.PanelKey == "" {
						cfg.PanelKey = legacy.PanelKey
					}
					if cfg.PanelBase == "" {
						cfg.PanelBase = legacy.PanelBase
					}
					if legacy.MailDomain != "" {
						cfg.MailDomain = legacy.MailDomain
					}
					if cfg.ImapHost == "" {
						cfg.ImapHost = legacy.ImapHost
					}
					if cfg.ImapPort == 0 {
						cfg.ImapPort = legacy.ImapPort
					}
					if cfg.ImapUser == "" {
						cfg.ImapUser = legacy.ImapUser
					}
					if cfg.ImapPassword == "" {
						cfg.ImapPassword = legacy.ImapPassword
					}
					if cfg.ProxyURL == "" {
						cfg.ProxyURL = legacy.ProxyURL
					}
				}
			}
		}
	}
	return cfg
}

func runCLITestOne() {
	cfg := loadCLIConfig()
	cfg.MailMode = "imap"
	cfg.Count = 1
	cfg.OutputDir = filepath.Join(detectWorkspace(), "artifacts", "go-protocol-test")
	_ = os.Remove(filepath.Join(cfg.OutputDir, "summary.json"))
	cfg.applyDefaults()

	runner := NewProtocolRunner(
		func(level, msg string) {
			fmt.Printf("[%s] [%s] %s\n", time.Now().Format("15:04:05"), level, msg)
		},
		nil,
		nil,
	)
	if err := runner.Start(cfg); err != nil {
		log.Fatalf("启动失败: %v", err)
	}
	for runner.IsRunning() {
		time.Sleep(1 * time.Second)
	}
	sum := runner.Summary()
	fmt.Printf("TEST_DONE succeeded=%d failed=%d\n", sum.Succeeded, sum.FailedAttempts)
	if sum.Succeeded < 1 {
		os.Exit(1)
	}
}

func runCLIBatch(count int, workers int) {
	cfg := loadCLIConfig()
	if n, err := RecycleReserved(cfg.HotmailPool); err == nil && n > 0 {
		fmt.Printf("[start] 已回收残留 reserved 账号 %d 个\n", n)
	}
	if count > 0 {
		cfg.Count = count
	}
	if workers > 0 {
		cfg.Workers = workers
	}
	cfg.applyDefaults()

	runner := NewProtocolRunner(
		func(level, msg string) {
			fmt.Printf("[%s] [%s] %s\n", time.Now().Format("15:04:05"), level, msg)
		},
		func(s BatchSummary) {
			fmt.Printf("[%s] [state] 成功 %d/%d，失败 %d，阶段：%s\n", time.Now().Format("15:04:05"), s.Succeeded, s.Target, s.FailedAttempts, s.CurrentStage)
		},
		nil,
	)
	if err := runner.Start(cfg); err != nil {
		log.Fatalf("启动失败: %v", err)
	}
	for runner.IsRunning() {
		time.Sleep(2 * time.Second)
	}
	sum := runner.Summary()
	fmt.Printf("BATCH_DONE succeeded=%d failed=%d target=%d\n", sum.Succeeded, sum.FailedAttempts, sum.Target)
	if sum.Succeeded < 1 {
		os.Exit(1)
	}
}

// runCLISupervised 监督模式：异常自动重启续跑，直到成功数达标或号池耗尽。Ctrl+C 优雅停止。
func runCLISupervised(count, workers int) {
	cfg := loadCLIConfig()
	if count > 0 {
		cfg.Count = count
	}
	if workers > 0 {
		cfg.Workers = workers
	}
	cfg.applyDefaults()

	stop := make(chan struct{})
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	go func() {
		<-sigCh
		close(stop)
	}()

	result, err := RunSupervisedBatch(cfg, stop, func(line string) {
		fmt.Println(line)
	})
	if err != nil {
		log.Fatalf("监督循环异常: %v", err)
	}
	fmt.Printf("SUPERVISED_DONE %s\n", result)
}
