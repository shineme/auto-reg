package main

import (
	"fmt"
	"path/filepath"
	"time"
)

// RunSupervisedBatch 持续跑批量注册：
// 每轮启动 runner（summary 持久化自动续跑），异常停止后等 20s 自动重启继续，
// 直到成功数达到目标、号池耗尽或收到手动停止信号。
func RunSupervisedBatch(cfg Config, stopCh <-chan struct{}, onLog func(string)) (string, error) {
	cfg.applyDefaults()
	_ = filepath.Join(cfg.OutputDir, "summary.json")

	stop := func(d time.Duration) bool {
		t := time.NewTimer(d)
		defer t.Stop()
		select {
		case <-stopCh:
			return true
		case <-t.C:
			return false
		}
	}

	for round := 1; ; round++ {
		if n, err := RecycleReserved(cfg.HotmailPool); err == nil && n > 0 {
			onLog(fmt.Sprintf("[supervised] 第 %d 轮：回收残留 reserved 账号 %d 个", round, n))
		}
		runner := NewProtocolRunner(
			func(level, msg string) {
				onLog(fmt.Sprintf("[%s] [%s] %s", time.Now().Format("15:04:05"), level, msg))
			},
			func(s BatchSummary) {
				onLog(fmt.Sprintf("[%s] [state] 成功 %d/%d，失败尝试 %d，阶段：%s",
					time.Now().Format("15:04:05"), s.Succeeded, s.Target, s.FailedAttempts, s.CurrentStage))
			},
		)
		if err := runner.Start(cfg); err != nil {
			onLog(fmt.Sprintf("[supervised] 第 %d 轮启动失败: %v，60s 后重试", round, err))
			if stop(60 * time.Second) {
				return "手动停止", nil
			}
			continue
		}
		for runner.IsRunning() {
			select {
			case <-stopCh:
				runner.Stop()
				onLog("[supervised] 收到停止信号")
				return "手动停止", nil
			default:
				time.Sleep(2 * time.Second)
			}
		}
		sum := runner.Summary()
		onLog(fmt.Sprintf("[supervised] 第 %d 轮结束：成功 %d/%d，失败尝试 %d", round, sum.Succeeded, sum.Target, sum.FailedAttempts))
		if sum.Succeeded >= sum.Target {
			return fmt.Sprintf("达成目标：成功 %d/%d", sum.Succeeded, sum.Target), nil
		}
		stats, err := loadPoolStats(cfg.HotmailPool)
		if err == nil && stats.Available+stats.Reserved == 0 {
			onLog("[supervised] 号池已耗尽，停止监督循环")
			return fmt.Sprintf("号池耗尽：成功 %d（目标 %d）", sum.Succeeded, sum.Target), nil
		}
		if stop(20 * time.Second) {
			return "手动停止", nil
		}
		onLog("[supervised] 20s 后自动重启继续…")
	}
}

func loadPoolStats(poolPath string) (HotmailPoolStats, error) {
	pool, err := LoadHotmailPool(poolPath)
	if err != nil {
		return HotmailPoolStats{}, err
	}
	return pool.Stats(), nil
}
