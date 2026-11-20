import io
p = r'C:\Users\dhl\Desktop\m3\go-protocol\cursor_flow.go'
s = io.open(p, encoding='utf-8').read()
old = """		_, _ = b.sendWait(ctx, "Input.dispatchMouseEvent", map[string]any{
			"type": typ, "x": x, "y": y, "button": button, "clickCount": 1,
		})"""
new = """		payload := map[string]any{
			"type": typ, "x": x, "y": y, "button": button, "clickCount": 1,
		}
		if typ != "mouseMoved" {
			payload["buttons"] = 1
		}
		_, _ = b.sendWait(ctx, "Input.dispatchMouseEvent", payload)"""
assert old in s
io.open(p, 'w', encoding='utf-8', newline='').write(s.replace(old, new, 1))
print('ok')
