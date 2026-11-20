import io
p = r'C:\Users\dhl\Desktop\m3\go-protocol\cursor_flow.go'
s = io.open(p, encoding='utf-8').read()
old = """	clicked, _ := b.clickUI(ctx, cursorCodeSubmitWords...)
	if clicked == "" {
		clicked, _ = b.submitUI(ctx)
	}
	return clicked, nil
}"""
new = """	clicked, _ := b.clickUI(ctx, cursorCodeSubmitWords...)
	if clicked == "" {
		// 没有明确提交按钮就再回车一次，避免误点“重新发送”。
		for _, typ := range []string{"keyDown", "char", "keyUp"} {
			payload := map[string]any{
				"type": typ, "key": "Enter", "code": "Enter",
				"windowsVirtualKeyCode": 13, "nativeVirtualKeyCode": 13,
			}
			if typ == "char" {
				payload["text"] = enterText
			}
			_, _ = b.sendWait(ctx, "Input.dispatchKeyEvent", payload)
		}
		time.Sleep(1500 * time.Millisecond)
	}
	return clicked, nil
}"""
assert old in s
io.open(p, 'w', encoding='utf-8', newline='').write(s.replace(old, new, 1))
print('ok')
