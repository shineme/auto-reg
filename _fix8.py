import io
p = r"C:\Users\dhl\Desktop\m3\go-protocol\cursor.go"
s = io.open(p, encoding="utf-8").read()
anchor = "func (b *cursorBrowser) evalJS(ctx context.Context, expr string, awaitPromise bool) (any, error) {"
helper = '''// send 发送一条 CDP 命令（不等待结果）。
func (b *cursorBrowser) send(method string, params map[string]any) {
	if b.conn == nil {
		return
	}
	b.msgID++
	if params == nil {
		params = map[string]any{}
	}
	raw, _ := json.Marshal(map[string]any{"id": b.msgID, "method": method, "params": params})
	_ = b.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	_ = b.conn.WriteMessage(websocket.TextMessage, raw)
}

'''
assert anchor in s
s = s.replace(anchor, helper + anchor)
old2 = '\tif _, err := browser.evalJS(ctx, cursorPageHelperJS, false); err != nil {'
io.open(p, "w", encoding="utf-8").write(s)
print("ok")
