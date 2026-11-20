import io, sys
Q = chr(34)
p = r'C:\Users\dhl\Desktop\m3\go-protocol\cursor.go'
s = io.open(p, encoding='utf-8').read()
old = '\t\t' + Q + '--window-size=1440,900' + Q + ',\n\t}\n'
new = old + '\tif cursorManualHuman {\n\t\targs = append(args, ' + Q + '--window-position=40,40' + Q + ')\n\t} else {\n\t\targs = append(args, ' + Q + '--window-position=-32000,-32000' + Q + ')\n\t}\n'
if old not in s:
    print('NOT FOUND'); sys.exit(1)
io.open(p, 'w', encoding='utf-8', newline='').write(s.replace(old, new, 1))
print('ok')
