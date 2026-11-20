import io
p = r'C:\Users\dhl\Desktop\m3\go-protocol\mail_hotmail.go'
s = io.open(p, encoding='utf-8').read()
i = s.index('return link, nil')
k = s.index('\n\t\t\t\t\t\t}', i)
k2 = s.index('\n\t\t\t\t\t\t}', k + 1)
s = s[:k2] + s[k2 + len('\n\t\t\t\t\t\t}'):]
io.open(p, 'w', encoding='utf-8', newline='').write(s)
print('ok')
