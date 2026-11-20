import io
p = r'C:\Users\dhl\Desktop\m3\go-protocol\cursor_exec.go'
s = io.open(p, encoding='utf-8').read()
old = '\t\t\tif ok, _ := browser.clickAtReal(ctx, "Continue"); ok {'
new = '\t\t\tif ok, _ := browser.clickAtReal(ctx, "Continue"); ok {'
assert old in s
s = s.replace(old, new, 1)
io.open(p, 'w', encoding='utf-8', newline='').write(s)
print('ok')
