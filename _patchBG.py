import io
p = r'C:\\Users\\dhl\\Desktop\\m3\\go-protocol\\cursor_app.go'
code = '''
// StopCursorBinding 停止 Cursor 绑定任务。
func (a *App) StopCursorBinding() {
	cursorRunState.mu.Lock()
	defer cursorRunState.mu.Unlock()
	if cursorRunState.cancel != nil {
		cursorRunState.cancel()
		cursorRunState.cancel = nil
	}
}

// StartCursorBinding 用号池里的 Hotmail 账号跑 Cursor 授权绑定，成功 count 个后收工。
func (a *App) StartCursorBinding(cfg Config, count int) error {
	cfg.applyDefaults()
	if err := SaveConfig(cfg); err != nil {
		return err
	}
	if count <= 0 {
		count = 1
	}
	cursorRunState.mu.Lock()
	if cursorRunState.running {
		cursorRunState.mu.Unlock()
		return fmt.Errorf("Cursor 绑定已在运行中")
	}
	base := a.ctx
	if base == nil {
		base = context.Background()
	}
	ctx, cancel := context.WithCancel(base)
	cursorRunState.cancel = cancel
	cursorRunState.running = true
	cursorRunState.mu.Unlock()
	go a.runCursorBinding(ctx, cfg, count)
	return nil
}
'''
io.open(p,'a',encoding='utf-8',newline='').write(code)
print('ok')
