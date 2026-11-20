import io
p = r'C:\Users\dhl\Desktop\m3\go-protocol\cursor_flow.go'
s = io.open(p, encoding='utf-8').read()
code = """
// browserSend 发一条浏览器级 CDP 命令并等它的结果。
func (b *cursorBrowser) browserSend(ctx context.Context, method string, params map[string]any, sessionID string) (map[string]any, error) {
	if b.bws == nil {
		return nil, fmt.Errorf("浏览器级连接不可用")
	}
	b.bmsgID++
	id := b.bmsgID
	payload := map[string]any{"id": id, "method": method, "params": params}
	if sessionID != "" {
		payload["sessionId"] = sessionID
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	_ = b.bws.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if err := b.bws.WriteMessage(websocket.TextMessage, data); err != nil {
		return nil, err
	}
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		_ = b.bws.SetReadDeadline(time.Now().Add(20 * time.Second))
		_, raw, err := b.bws.ReadMessage()
		if err != nil {
			return nil, err
		}
		var msg map[string]any
		if json.Unmarshal(raw, &msg) != nil {
			continue
		}
		if mid, ok := msg["id"].(float64); ok && int(mid) == id {
			return msg, nil
		}
	}
	return nil, fmt.Errorf("%s 超时", method)
}
"""
io.open(p, 'w', encoding='utf-8', newline='').write(s + code)
print('ok')
