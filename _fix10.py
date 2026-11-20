import io
p = r"C:\Users\dhl\Desktop\m3\go-protocol\cursor_flow.go"
s = io.open(p, encoding="utf-8").read()
old = 'case "Runtime.consoleAPICalled", "Runtime.exceptionThrown", "Network.loadingFailed":'
new = 'case "Runtime.consoleAPICalled", "Runtime.exceptionThrown", "Network.loadingFailed", "Network.responseReceived":'
assert old in s
s = s.replace(old, new)
io.open(p, "w", encoding="utf-8").write(s)
print("ok")
