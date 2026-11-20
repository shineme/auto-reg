import io
p = r"C:\Users\dhl\Desktop\m3\go-protocol\cursor_cli.go"
s = io.open(p, encoding="utf-8").read()
old = """\tres, err := flow.Execute(ctx, acc.Email, fetchCode, func(step string) {
\t\tfmt.Printf("[cursor] %s\\n", step)
\t})
"""
new = """\tvar res *CursorResult
\tattempts := 3
\tfor attempt := 1; attempt <= attempts; attempt++ {
\t\tif attempt > 1 {
\t\t\tfmt.Printf("[cursor] 第 %d 次尝试：换出口 IP 重试\\n", attempt)
\t\t\tif dialer != nil {
\t\t\t\tdialer.RotateSession()
\t\t\t}
\t\t\ttime.Sleep(5 * time.Second)
\t\t}
\t\tvar aerr error
\t\tres, aerr = flow.Execute(ctx, acc.Email, fetchCode, func(step string) {
\t\t\tfmt.Printf("[cursor] %s\\n", step)
\t\t})
\t\terr = aerr
\t\tif err == nil {
\t\t\tbreak
\t\t}
\t\tfmt.Printf("[cursor] 第 %d 次尝试失败: %v\\n", attempt, err)
\t}
"""
assert old in s
s = s.replace(old, new)
io.open(p, "w", encoding="utf-8").write(s)
print("ok")
