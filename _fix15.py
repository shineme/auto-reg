import io
p = r"C:\Users\dhl\Desktop\m3\go-protocol\main.go"
s = io.open(p, encoding="utf-8").read()
s = s.replace("//go:build !web", "//go:build !web && !cli")
io.open(p, "w", encoding="utf-8").write(s)
print("ok")
