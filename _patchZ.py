import io
p = r'C:\Users\dhl\Desktop\m3\go-protocol\cursor_flow.go'
s = io.open(p, encoding='utf-8').read()
old = 'text: body.slice(0, 600),'
new = 'text: body.slice(0, 2000),'
assert old in s
io.open(p, 'w', encoding='utf-8', newline='').write(s.replace(old, new, 1))
print('ok')
