import io
p = r"C:\Users\dhl\Desktop\m3\go-protocol\cursor_exec.go"
s = io.open(p, encoding="utf-8").read()
anchor = "\t\t// 密码页：新账号需要设密码"
block = """\t\t// 人机验证：Turnstile 复选框要点一下
\t\tif strings.Contains(st.Text, "sure you are human") || strings.Contains(st.Text, "verify the user is human") || strings.Contains(st.Text, "人机") {
\t\t\tif clicked, _ := browser.clickTurnstile(ctx); clicked {
\t\t\t\tcursorDebug("human check handled")
\t\t\t\ttime.Sleep(4 * time.Second)
\t\t\t\tcontinue
\t\t\t}
\t\t}

"""
assert anchor in s
s = s.replace(anchor, block + anchor)
io.open(p, "w", encoding="utf-8").write(s)
print("ok")
