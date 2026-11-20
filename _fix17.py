import io
p = r"C:\Users\dhl\Desktop\m3\go-protocol\cursor.go"
s = io.open(p, encoding="utf-8").read()
anchor = "// send 发送一条 CDP 命令（不等待结果）。"
helper = '''// sendWait 发送一条 CDP 命令并等待它的响应。
func (b *cursorBrowser) sendWait(ctx context.Context, method string, params map[string]any) (map[string]any, error) {
	if b.conn == nil {
		return nil, fmt.Errorf("浏览器连接未建立")
	}
	b.msgID++
	if params == nil {
		params = map[string]any{}
	}
	raw, _ := json.Marshal(map[string]any{"id": b.msgID, "method": method, "params": params})
	_ = b.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if err := b.conn.WriteMessage(websocket.TextMessage, raw); err != nil {
		return nil, err
	}
	for {
		_ = b.conn.SetReadDeadline(time.Now().Add(20 * time.Second))
		_, data, err := b.conn.ReadMessage()
		if err != nil {
			return nil, err
		}
		var resp map[string]any
		if json.Unmarshal(data, &resp) != nil {
			continue
		}
		if meth, _ := resp["method"].(string); meth != "" {
			cursorCDPEvent(meth, data)
			continue
		}
		if id, _ := resp["id"].(float64); int(id) == b.msgID {
			return resp, nil
		}
	}
}

'''
assert anchor in s
s = s.replace(anchor, helper + anchor)
io.open(p, "w", encoding="utf-8").write(s)
print("ok")
