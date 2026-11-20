import io
p = r'C:\Users\dhl\Desktop\m3\go-protocol\mail_hotmail.go'
s = io.open(p, encoding='utf-8').read()
old = '\tcase "cursor":\n\t\treturn strings.Contains(sender, "cursor") || strings.Contains(subject, "cursor") || strings.Contains(subject, "登录 cursor")'
new = ('\tcase "cursor":\n'
       '\t\tif strings.Contains(subject, "confirm a change") {\n'
       '\t\t\treturn false\n'
       '\t\t}\n'
       '\t\treturn strings.Contains(sender, "cursor") || strings.Contains(subject, "cursor") || strings.Contains(subject, "登录 cursor")\n'
       '\tcase "cursor-confirm":\n'
       '\t\treturn strings.Contains(subject, "confirm") || strings.Contains(subject, "确认")')
assert old in s
io.open(p, 'w', encoding='utf-8', newline='').write(s.replace(old, new, 1))
print('ok')
