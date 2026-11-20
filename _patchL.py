import io
p = r'C:\Users\dhl\Desktop\m3\go-protocol\cursor_exec.go'
s = io.open(p, encoding='utf-8').read()
anchor = '\t\t// 面板已落库就收工\n'
new = """\t\t// 面板 sign-in 记录变 done 就收工（账号早就绑过时也能认出来）。
\t\tif state, user, err := f.Panel.CursorSignInState(ctx, signIn.ID); err == nil {
\t\t\tif state == "done" && user != "" {
\t\t\t\treturn &CursorResult{SignInID: signIn.ID, User: user, Email: email}, nil
\t\t\t}
\t\t\tif state == "error" || state == "expired" || state == "failed" {
\t\t\t\treturn nil, fmt.Errorf("面板 sign-in 终态异常: %s", state)
\t\t\t}
\t\t}
"""
assert anchor in s
io.open(p, 'w', encoding='utf-8', newline='').write(s.replace(anchor, new + anchor, 1))
print('ok')
