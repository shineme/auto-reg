import io
p = r'C:\Users\dhl\Desktop\m3\go-protocol\cursor_flow.go'
s = io.open(p, encoding='utf-8').read()
old = '\tif os.Getenv("MINIMAX_CURSOR_DEBUG") != "1" {\n\t\treturn\n\t}\n\tswitch method {'
new = '\tif os.Getenv("MINIMAX_CURSOR_CDP") != "1" {\n\t\treturn\n\t}\n\tswitch method {'
assert old in s
io.open(p, 'w', encoding='utf-8', newline='').write(s.replace(old, new, 1))
print('ok')
