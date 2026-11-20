import io
p = r"C:\Users\dhl\Desktop\m3\go-protocol\cursor.go"
s = io.open(p, encoding="utf-8").read()
old = 'if id, _ := resp["id"].(float64); int(id) != b.msgID {'
new = 'if meth, _ := resp["method"].(string); meth != "" {\n\t\t\tcursorCDPEvent(meth, data)\n\t\t\tcontinue\n\t\t}\n\t\t' + old
assert old in s
s = s.replace(old, new)
io.open(p, "w", encoding="utf-8").write(s)
print("ok")
