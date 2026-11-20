import io
p = r'C:\Users\dhl\Desktop\m3\go-protocol\cursor_exec.go'
s = io.open(p, encoding='utf-8').read()
old = """			if clicked, _ := browser.clickOut(ctx, "Sign in"); clicked != "" {"""
new = """			if ok, _ := browser.clickAtReal(ctx, "Sign in"); ok {"""
assert old in s
io.open(p, 'w', encoding='utf-8', newline='').write(s.replace(old, new, 1))
print('ok')
