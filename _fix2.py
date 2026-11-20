import io
p = r"C:\Users\dhl\Desktop\m3\go-protocol\cursor_cli.go"
s = io.open(p, encoding="utf-8").read()
old = """	bridgeAddr := ""
	if dialer != nil && dialer.Enabled() {"""
new = """	bridgeAddr := ""
	if dialer != nil && dialer.Enabled() && !cursorDirectProxy {"""
assert old in s
s = s.replace(old, new)
old2 = "func runCLITestCursor(panelBase, panelKey string) {"
new2 = """// cursorDirectProxy 为 true 时 Chrome 不走本地代理桥，直接用系统代理。
var cursorDirectProxy = os.Getenv("MINIMAX_CURSOR_DIRECT") == "1"

func runCLITestCursor(panelBase, panelKey string) {"""
assert old2 in s
s = s.replace(old2, new2)
io.open(p, "w", encoding="utf-8").write(s)
print("ok")
