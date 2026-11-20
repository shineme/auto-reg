import io
p = r"C:\Users\dhl\Desktop\m3\go-protocol\cursor_flow.go"
s = io.open(p, encoding="utf-8").read()
helper = '''
// cursorCDPEvent 记录浏览器控制台错误和网络失败，调试用。
func cursorCDPEvent(method string, data []byte) {
	if os.Getenv("MINIMAX_CURSOR_DEBUG") != "1" {
		return
	}
	switch method {
	case "Runtime.consoleAPICalled", "Runtime.exceptionThrown", "Network.loadingFailed":
		if len(data) > 600 {
			data = data[:600]
		}
		fmt.Printf("[cursor-cdp] %s %s\\n", method, string(data))
	}
}
'''
s = s.rstrip() + "\n" + helper
io.open(p, "w", encoding="utf-8").write(s)
print("ok")
