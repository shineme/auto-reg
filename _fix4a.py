import io
p = r"C:\Users\dhl\Desktop\m3\go-protocol\cursor_exec.go"
s = io.open(p, encoding="utf-8").read()
old = "cursorDebug(\"read state failed: %v\", err)"
new = "cursorDebug(\"read state failed: %v\", err)"
print(old in s)
