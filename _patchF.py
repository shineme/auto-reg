import io
p = r'C:\Users\dhl\Desktop\m3\go-protocol\cursor_exec.go'
s = io.open(p, encoding='utf-8').read()
a = s.index('\t\t\tok, err := browser.setUI(ctx, "code", verifyCode)')
b = s.index('\t\t// 邮箱页')
new = """\t\t\tclicked, err := browser.fillCode(ctx, verifyCode)
\t\t\tif err != nil {
\t\t\t\treturn nil, err
\t\t\t}
\t\t\tif onStep != nil {
\t\t\t\tonStep("填写验证码")
\t\t\t}
\t\t\tcursorDebug("code submitted via %q", clicked)
\t\t\tcodeTries++
\t\t\tif onStep != nil {
\t\t\t\tonStep("提交登录")
\t\t\t}
\t\t\ttime.Sleep(4 * time.Second)
\t\t\tcontinue
\t\t}

"""
io.open(p, 'w', encoding='utf-8', newline='').write(s[:a] + new + s[b:])
print('ok')
