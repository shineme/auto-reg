import io
p = r"C:\Users\dhl\Desktop\m3\go-protocol\cursor_flow.go"
s = io.open(p, encoding="utf-8").read()
old = "Math.round(i.getBoundingClientRect().width)].join('|'); });"
new = "Math.round(i.getBoundingClientRect().width), (i.value || '').length].join('|'); });"
assert old in s
s = s.replace(old, new)
io.open(p, "w", encoding="utf-8").write(s)
print("ok")
