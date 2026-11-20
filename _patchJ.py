import io
p = r'C:\Users\dhl\Desktop\m3\go-protocol\cursor_exec.go'
s = io.open(p, encoding='utf-8').read()
anchor = '\n// 密码页'
new = """
\t\t// Cookie 提示条：先关掉，免得挡住后面的按钮。
\t\tif clicked, _ := browser.clickOut(ctx, "Dismiss cookie notice"); clicked != "" {
\t\t\tcursorDebug("cookie notice dismissed")
\t\t\ttime.Sleep(1 * time.Second)
\t\t\tcontinue
\t\t}
\t\t// 授权确认页：点 Sign in 完成授权。
\t\tif strings.Contains(st.Text, "Before you approve") || strings.Contains(st.Text, "asking to sign in with this account") {
\t\t\tif clicked, _ := browser.clickOut(ctx, "Sign in"); clicked != "" {
\t\t\t\tif onStep != nil {
\t\t\t\t\tonStep("确认授权")
\t\t\t\t}
\t\t\t\tcursorDebug("approved sign-in request")
\t\t\t\ttime.Sleep(3 * time.Second)
\t\t\t\tcontinue
\t\t\t}
\t\t}
"""
assert anchor in s
io.open(p, 'w', encoding='utf-8', newline='').write(s.replace(anchor, new + anchor, 1))
print('ok')
