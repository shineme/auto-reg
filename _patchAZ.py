import io
p = r'C:\Users\dhl\Desktop\m3\go-protocol\cursor_exec.go'
s = io.open(p, encoding='utf-8').read()
old = '\t\t\t\tfmt.Println("[cursor] 请在 Chrome 窗口点击人机验证（窗口已置于屏幕左上角）")'
new = old + '\n\t\t\t\tfocusCursorWindow(browser.ProfileDir)'
assert old in s
io.open(p, 'w', encoding='utf-8', newline='').write(s.replace(old, new, 1))
print('ok')
