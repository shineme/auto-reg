import io
p = r'C:\\Users\\dhl\\Desktop\\m3\\go-protocol\\cursor_app.go'
code = '''	poolPath, err := prepareCursorTestPool(cfg)
	if err != nil {
		a.emitBatchLog("error", "[cursor] 准备号池失败: "+err.Error())
		return
	}
	store := NewHotmailPoolStore(poolPath)
	panel, err := NewPanelClient(panelBase, panelKey, dialer, cfg.ProxyDirectHosts)
	if err == nil {
		err = panel.Login(ctx)
	}
	if err != nil {
		a.emitBatchLog("error", "[cursor] 面板登录失败: "+err.Error())
		return
	}
	mailClient := &http.Client{Timeout: 30 * time.Second}
	for ok < count {
		if ctx.Err() != nil {
			return
		}
		acc, idx, cerr := store.Claim()
		if cerr != nil {
			a.emitBatchLog("error", "[cursor] 号池取号失败: "+cerr.Error())
			return
		}
'''
io.open(p,'a',encoding='utf-8',newline='').write(code)
print('ok')
