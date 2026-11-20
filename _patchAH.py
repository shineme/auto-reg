import io
p = r'C:\Users\dhl\Desktop\m3\go-protocol\mail_hotmail.go'
s = io.open(p, encoding='utf-8').read()
old = '\t\t\t\t\t\tfor _, link := range linkPattern.FindAllString(msg.BodyPreview+" "+rawBody, -1) {'
i = s.index(old)
j = s.index('\t\t\t\t\t\t}', i) + len('\t\t\t\t\t\t}')
new = ('\t\t\t\t\t\tcands := linkPattern.FindAllString(msg.BodyPreview+" "+rawBody, -1)\n'
       '\t\t\t\t\t\tfor _, lk := range cands {\n'
       '\t\t\t\t\t\t\tfmt.Printf("[cursor-mail] %s | %s | %s\\n", msg.ReceivedDateTime, msg.Subject, lk)\n'
       '\t\t\t\t\t\t}\n'
       '\t\t\t\t\t\tfor _, link := range cands {\n'
       '\t\t\t\t\t\t\tif strings.Contains(strings.ToLower(link), "cursor") {\n'
       '\t\t\t\t\t\t\t\treturn link, nil\n'
       '\t\t\t\t\t\t\t}\n'
       '\t\t\t\t\t\t}')
io.open(p, 'w', encoding='utf-8', newline='').write(s[:i] + new + s[j:])
print('ok')
