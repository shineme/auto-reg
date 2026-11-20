import io, sys
p = r'C:\Users\dhl\Desktop\m3\go-protocol\cursor_exec.go'
s = io.open(p, encoding='utf-8').read()
old = '\tswitchTries := 0\n'
new = '\tswitchTries := 0\n\thumanHinted := false\n'
if old not in s:
    print('NOT FOUND'); sys.exit(1)
io.open(p, 'w', encoding='utf-8', newline='').write(s.replace(old, new, 1))
print('ok')
