import io
p = r'C:\Users\dhl\Desktop\m3\go-protocol\cursor_exec.go'
s = io.open(p, encoding='utf-8').read()
old = '\t\t\t\tfocusCursorWindow(browser.ProfileDir)'
new = '\t\t\t\tif browser.cmd != nil && browser.cmd.Process != nil {\n\t\t\t\t\tfocusCursorWindow(uint32(browser.cmd.Process.Pid))\n\t\t\t\t}'
assert old in s
io.open(p, 'w', encoding='utf-8', newline='').write(s.replace(old, new, 1))
print('ok')
