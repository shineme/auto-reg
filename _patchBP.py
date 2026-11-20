import io
p = r'C:\\Users\\dhl\\Desktop\\m3\\go-protocol\\cursor_app.go'
s = io.open(p, encoding='utf-8').read()
old = '''	bridgeAddr := ""
	if dialer != nil && dialer.Enabled() {
		if bridge, addr, err := StartLocalHTTPBridgeWithDialer(dialer); err == nil {
			defer bridge.Close()
			bridgeAddr = addr
		}
	}
'''
new = '''	// 浏览器走系统代理直连（本地代理桥对 cursor.com 的 CONNECT 常被上游拒绝），
	// 面板与 Graph 邮箱请求仍走配置的上游代理。
	bridgeAddr := ""
	a.emitBatchLog("info", "[cursor] 浏览器走系统代理，面板与邮箱请求走配置代理")
'''
assert old in s
io.open(p,'w',encoding='utf-8',newline='').write(s.replace(old, new, 1))
print('ok')
