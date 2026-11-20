import io
p = r'C:\Users\dhl\Desktop\m3\go-protocol\mail_hotmail.go'
s = io.open(p, encoding='utf-8').read()
old = '\t\t\t\t\t\tfor _, link := range cands {\n\t\t\t\t\t\t\tif strings.Contains(strings.ToLower(link), "cursor") {'
new = ('\t\t\t\t\t\tfor _, link := range cands {\n'
       '\t\t\t\t\t\t\tif strings.Contains(strings.ToLower(link), "confirm-action") {\n'
       '\t\t\t\t\t\t\t\treturn link, nil\n'
       '\t\t\t\t\t\t\t}\n'
       '\t\t\t\t\t\t}\n'
       '\t\t\t\t\t\tfor _, link := range cands {\n'
       '\t\t\t\t\t\t\tif strings.Contains(strings.ToLower(link), "cursor") {')
assert old in s
io.open(p, 'w', encoding='utf-8', newline='').write(s.replace(old, new, 1))
print('ok')
