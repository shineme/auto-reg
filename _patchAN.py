import io
p = r'C:\Users\dhl\Desktop\m3\go-protocol\cursor_flow.go'
s = io.open(p, encoding='utf-8').read()
code = """
// openNewTab 在同一个浏览器里新开一个标签页打开链接（确认链接必须留在原浏览器里）。
func (b *cursorBrowser) openNewTab(ctx context.Context, url string) error {
	if b.bws == nil {
		return fmt.Errorf("浏览器级连接不可用")
	}
	b.bmsgID++
	id := b.bmsgID
	payload, err := json.Marshal(map[string]any{
		"id": id, "method": "Target.createTarget", "params": map[string]any{"url": url},
	})
	if err != nil {
		return err
	}
	_ = b.bws.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if err := b.bws.WriteMessage(websocket.TextMessage, payload); err != nil {
		return err
	}
	return nil
}
"""
io.open(p, 'w', encoding='utf-8', newline='').write(s + code)
print('ok')
