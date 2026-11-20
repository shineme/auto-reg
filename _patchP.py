import io
p = r'C:\Users\dhl\Desktop\m3\go-protocol\cursor_exec.go'
s = io.open(p, encoding='utf-8').read()
code = """
// cursorPageWantsSignup 判断密码页是不是创建账号模式（已有账号登录时不该设密码）。
func cursorPageWantsSignup(st *cursorUIState) bool {
	for _, c := range st.CTA {
		l := strings.ToLower(c)
		if strings.Contains(l, "create account") || strings.Contains(l, "sign up") ||
			strings.Contains(l, "创建账号") || strings.Contains(l, "注册") {
			return true
		}
	}
	return false
}
"""
io.open(p, 'w', encoding='utf-8', newline='').write(s + code)
print('ok')
