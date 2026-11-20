import io
p = r'C:\Users\dhl\Desktop\m3\go-protocol\cursor_flow.go'
s = io.open(p, encoding='utf-8').read()
code = """
// fillCode 填入验证码：写入 OTP 输入框后发真实回车事件，再尝试点提交按钮。
func (b *cursorBrowser) fillCode(ctx context.Context, code string) (string, error) {
	v, err := b.evalJS(ctx, fmt.Sprintf("window.__mmFillCode(%q)", code), false)
	if err != nil {
		return "", err
	}
	ok, _ := v.(bool)
	if !ok {
		return "", fmt.Errorf("找不到验证码输入框")
	}
	enterText := string(rune(13))
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
	clicked, _ := b.clickUI(ctx, cursorCodeSubmitWords...)
	if clicked == "" {
		clicked, _ = b.submitUI(ctx)
	}
	return clicked, nil
}
"""
io.open(p, 'w', encoding='utf-8', newline='').write(s + code)
print('ok')
