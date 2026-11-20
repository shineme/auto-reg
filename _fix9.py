import io
p = r"C:\Users\dhl\Desktop\m3\go-protocol\cursor_exec.go"
s = io.open(p, encoding="utf-8").read()
old = '\t\t\tclicked, _ := browser.clickUI(ctx, "验证码", "verification code", "邮箱登录", "magic code", "email code")'
new = ('\t\t\tclicked, _ := browser.clickSel(ctx, `button[value="magic-code"]`)\n'
       '\t\t\tif clicked == "" {\n'
       '\t\t\t\tclicked, _ = browser.clickUI(ctx, "验证码", "verification code", "邮箱登录")\n'
       '\t\t\t}')
assert old in s
s = s.replace(old, new)
io.open(p, "w", encoding="utf-8").write(s)
print("ok")
