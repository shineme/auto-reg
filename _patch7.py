import io, sys
Q = chr(34)
p = r'C:\Users\dhl\Desktop\m3\go-protocol\cursor.go'
s = io.open(p, encoding='utf-8').read()
a = s.index('\tif cursorManualHuman {')
b = s.index('\tif runtime.GOOS != ')
new = ('\t// 窗口摆在屏幕左上角，人机验证由用户自己点。\n'
 '\targs = append(args, ' + Q + '--window-position=40,40' + Q + ')\n')
io.open(p, 'w', encoding='utf-8', newline='').write(s[:a] + new + s[b:])
print('ok')
