import io, re
p = r"C:\Users\dhl\Desktop\m3\go-protocol\cursor.go"
s = io.open(p, encoding="utf-8").read()

# 1) CursorFlow 增加 Password 字段
old_struct = """type CursorFlow struct {
	Panel      *PanelClient
	ChromePath string
	ProxyBidge string
	ProfileDir string
}"""
new_struct = """type CursorFlow struct {
	Panel      *PanelClient
	ChromePath string
	ProxyBidge string
	ProfileDir string
	// Password 新建 Cursor 账号时设置的密码，留空则用随机强密码。
	Password string
}"""
assert old_struct in s, "struct not found"
s = s.replace(old_struct, new_struct)

# 2) 删掉旧的 Run 方法（已被 cursor_exec.go 的 Execute 取代）
i = s.find("// Run 执行完整流程：面板申请授权")
assert i > 0, "Run not found"
s = s[:i].rstrip() + "\n"

io.open(p, "w", encoding="utf-8").write(s)
print("cursor.go patched")
