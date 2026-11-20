import io
p = r'C:\Users\dhl\Desktop\m3\go-protocol\cursor_exec.go'
s = io.open(p, encoding='utf-8').read()
old = '\t\t\tif !humanHinted {\n\t\t\t\thumanHinted = true'
new = '\t\t\tif !humanHinted || time.Since(lastHumanHint) > 15*time.Second {\n\t\t\t\tlastHumanHint = time.Now()\n\t\t\t\thumanHinted = true'
assert old in s
s = s.replace(old, new, 1)
s = s.replace('\thumanHinted := false\n', '\thumanHinted := false\n\tlastHumanHint := time.Time{}\n', 1)
io.open(p, 'w', encoding='utf-8', newline='').write(s)
print('ok')
