import io
p = r'C:\\Users\\dhl\\Desktop\\m3\\go-protocol\\cursor_app.go'
code = '''		flow := &CursorFlow{
			Panel:      panel,
			ChromePath: cfg.ChromePath,
			ProxyBidge: bridgeAddr,
			ProfileDir: filepath.Join(cursorTestDir(cfg), "chrome-profiles"),
			FetchLink: func(ctx context.Context, notBefore time.Time) (string, error) {
				return PollHotmailLink(ctx, mailClient, acc, notBefore, 3*time.Minute, "cursor-confirm")
			},
		}
		res, rerr := flow.Execute(ctx, acc.Email, func(ctx context.Context, notBefore time.Time) (string, error) {
			return PollHotmailCodeFor(ctx, mailClient, acc, notBefore, 3*time.Minute, "cursor")
		}, func(step string) {
			a.emitBatchLog("info", "["+acc.Email+"] "+step)
		})
		if rerr != nil {
			fail++
			store.Finalize(idx, acc.Email, "available", rerr.Error(), "")
			a.emitBatchLog("error", "["+acc.Email+"] "+rerr.Error())
		} else {
			ok++
			store.Finalize(idx, acc.Email, "used", "", acc.RefreshToken)
			a.emitBatchLog("info", fmt.Sprintf("[cursor] 成功 %d/%d：%s -> 面板账号 %s", ok, count, acc.Email, res.User))
		}
		emitEvent("batch:progress", ProgressLite{
			Target: count, Succeeded: ok, FailedAttempts: fail,
			CurrentStage: "Cursor 授权绑定", CurrentEmail: acc.Email,
			Running: true, UpdatedAt: time.Now(),
		})
	}
}
'''
io.open(p,'a',encoding='utf-8',newline='').write(code)
print('ok')
