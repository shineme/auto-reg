import io
p = r'C:\Users\dhl\Desktop\m3\go-protocol\cursor_flow.go'
s = io.open(p, encoding='utf-8').read()
code = """
// clickOut 点按文案完全一致的按钮，跳过 Cookie 同意框里的元素。
func (b *cursorBrowser) clickOut(ctx context.Context, text string) (string, error) {
	v, err := b.evalJS(ctx, fmt.Sprintf("window.__mmClickOut(%q)", text), false)
	if err != nil {
		return "", err
	}
	s, _ := v.(string)
	return s, nil
}
"""
io.open(p, 'w', encoding='utf-8', newline='').write(s + code)
print('ok')
