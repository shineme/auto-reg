import io
p = r"C:\Users\dhl\Desktop\m3\go-protocol\cursor_flow.go"
s = io.open(p, encoding="utf-8").read()
old = "\tm, ok := v.(map[string]any)\n\tif !ok {\n\t\treturn nil, fmt.Errorf(\"页面状态读取失败\")\n\t}"
new = "\tm, ok := v.(map[string]any)\n\tif !ok {\n\t\tif _, ierr := b.evalJS(ctx, cursorUIHelperJS, false); ierr == nil {\n\t\t\treturn nil, fmt.Errorf(\"页面脚本已重新注入，稍后重试\")\n\t\t}\n\t\treturn nil, fmt.Errorf(\"页面状态读取失败\")\n\t}"
assert old in s
s = s.replace(old, new)
io.open(p, "w", encoding="utf-8").write(s)
print("ok")
