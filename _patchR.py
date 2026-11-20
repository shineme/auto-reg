import io
p = r'C:\Users\dhl\Desktop\m3\go-protocol\cursor_flow.go'
s = io.open(p, encoding='utf-8').read()
code = """
// clickAtReal 用真实鼠标事件点按文案完全一致的按钮（部分页面只认 trusted 事件）。
func (b *cursorBrowser) clickAtReal(ctx context.Context, text string) (bool, error) {
	v, err := b.evalJS(ctx, fmt.Sprintf("window.__mmRect(%q)", text), false)
	if err != nil {
		return false, err
	}
	m, ok := v.(map[string]any)
	if !ok {
		return false, nil
	}
	x, _ := m["x"].(float64)
	y, _ := m["y"].(float64)
	if x == 0 && y == 0 {
		return false, nil
	}
	for _, typ := range []string{"mouseMoved", "mousePressed", "mouseReleased"} {
		button := "left"
		if typ == "mouseMoved" {
			button = "none"
		}
		_, _ = b.sendWait(ctx, "Input.dispatchMouseEvent", map[string]any{
			"type": typ, "x": x, "y": y, "button": button, "clickCount": 1,
		})
	}
	return true, nil
}
"""
io.open(p, 'w', encoding='utf-8', newline='').write(s + code)
print('ok')
