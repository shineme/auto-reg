import io
p = r'C:\Users\dhl\Desktop\m3\go-protocol\cursor_exec.go'
s = io.open(p, encoding='utf-8').read()
old = '\t\t\t\tif nerr := browser.openNewTab(ctx, confirmLink); nerr != nil {\n\t\t\t\t\tcursorDebug("open tab failed: %v", nerr)\n\t\t\t\t}'
new = '\t\t\t\tif okc, nerr := browser.openConfirmTab(ctx, confirmLink); nerr != nil {\n\t\t\t\t\tcursorDebug("open tab failed: %v", nerr)\n\t\t\t\t} else if okc {\n\t\t\t\t\tcursorDebug("confirm action clicked")\n\t\t\t\t}'
assert old in s
io.open(p, 'w', encoding='utf-8', newline='').write(s.replace(old, new, 1))
print('ok')
