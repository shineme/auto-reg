import io
p = r"C:\Users\dhl\Desktop\m3\go-protocol\cursor_exec.go"
s = io.open(p, encoding="utf-8").read()
old = '\tbrowser.send("Runtime.enable", nil)\n\tbrowser.send("Network.enable", nil)\n\tif err != nil {\n\t\treturn nil, err\n\t}\n'
new = '\tif err != nil {\n\t\treturn nil, err\n\t}\n\tbrowser.send("Runtime.enable", nil)\n\tbrowser.send("Network.enable", nil)\n'
assert old in s
s = s.replace(old, new)
io.open(p, "w", encoding="utf-8").write(s)
print("ok")
