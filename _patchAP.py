import io
p = r'C:\Users\dhl\Desktop\m3\go-protocol\cursor_exec.go'
s = io.open(p, encoding='utf-8').read()
old = '\t\t\t\tbrowser.send("Page.navigate", map[string]any{"url": confirmLink})'
new = '\t\t\t\tif nerr := browser.openNewTab(ctx, confirmLink); nerr != nil {\n\t\t\t\t\tcursorDebug("open tab failed: %v", nerr)\n\t\t\t\t}'
assert old in s
io.open(p, 'w', encoding='utf-8', newline='').write(s.replace(old, new, 1))
print('ok')
