import io
p = r'C:\Users\dhl\Desktop\m3\go-protocol\mail_hotmail.go'
s = io.open(p, encoding='utf-8').read()
i = s.index('for _, link := range cands {')
j = s.index('}\n', s.index('return link, nil'))
print(repr(s[i:i+400]))
