import io
p = r"C:\Users\dhl\Desktop\m3\go-protocol\cursor_flow.go"
s = io.open(p, encoding="utf-8").read()
anchor = "// clickSel 按 CSS 选择器直接点击元素。"
helper = '''// clickTurnstile 点 Cloudflare Turnstile 的“我是人”复选框。
func (b *cursorBrowser) clickTurnstile(ctx context.Context) (bool, error) {
	v, err := b.evalJS(ctx, "(function(){var f=document.querySelector('iframe[src*=challenges]');if(!f)return null;var r=f.getBoundingClientRect();return {x:r.x,y:r.y,w:r.width,h:r.height};})()", false)
	if err != nil {
		return false, err
	}
	m, ok := v.(map[string]any)
	if !ok {
		return false, nil
	}
	x, _ := m["x"].(float64)
	y, _ := m["y"].(float64)
	w, _ := m["w"].(float64)
	h, _ := m["h"].(float64)
	if w <= 0 || h <= 0 {
		return false, nil
	}
	cx := x + 28
	cy := y + h/2
	if cy < y+10 {
		cy = y + 30
	}
	for _, typ := range []string{"mouseMoved", "mousePressed", "mouseReleased"} {
		button := "left"
		if typ == "mouseMoved" {
			button = "none"
		}
		_, _ = b.sendWait(ctx, "Input.dispatchMouseEvent", map[string]any{
			"type": typ, "x": cx, "y": cy, "button": button,
			"clickCount": 1,
		})
	}
	cursorDebug("turnstile clicked at %.0f,%.0f", cx, cy)
	return true, nil
}

'''
assert anchor in s
s = s.replace(anchor, helper + anchor)
io.open(p, "w", encoding="utf-8").write(s)
print("ok")
