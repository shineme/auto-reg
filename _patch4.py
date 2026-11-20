import io, sys
p = r'C:\Users\dhl\Desktop\m3\go-protocol\cursor.go'
s = io.open(p, encoding='utf-8').read()
old = 'b := &cursorBrowser{cmd: cmd, cancel: cancel}'
new = 'b := &cursorBrowser{cmd: cmd, cancel: cancel, profileDir: profileDir}'
if old not in s:
    print('NOT FOUND'); sys.exit(1)
io.open(p, 'w', encoding='utf-8', newline='').write(s.replace(old, new, 1))
print('ok')
