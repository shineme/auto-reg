import io
p = r'C:\\Users\\dhl\\Desktop\\m3\\go-protocol\\cursor_app.go'
code = '''
// runCursorBinding 循环跑 Cursor 绑定，直到成功 count 个、号池取空或被停止。
func (a *App) runCursorBinding(ctx context.Context, cfg Config, count int) {
	ok, fail := 0, 0
	defer func() {
		cursorRunState.mu.Lock()
		cursorRunState.running = false
		cursorRunState.mu.Unlock()
		a.emitBatchLog("info", fmt.Sprintf("[cursor] 已停止：成功 %d/%d，失败尝试 %d", ok, count, fail))
	}()
	panelBase := cfg.Panel2Base
	if panelBase == "" {
		panelBase = cfg.PanelBase
	}
	panelKey := cfg.Panel2Key
	if panelKey == "" {
		panelKey = cfg.PanelKey
	}
	var dialer *ProxyDialer
	if cfg.ProxyURL != "" {
		if d, err := NewProxyDialer(cfg.ProxyURL); err == nil {
			dialer = d
		}
	}
	bridgeAddr := ""
	if dialer != nil && dialer.Enabled() {
		if bridge, addr, err := StartLocalHTTPBridgeWithDialer(dialer); err == nil {
			defer bridge.Close()
			bridgeAddr = addr
		}
	}
'''
io.open(p,'a',encoding='utf-8',newline='').write(code)
print('ok')
