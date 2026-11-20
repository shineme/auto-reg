import io
p = r'C:\Users\dhl\Desktop\m3\go-protocol\mail_hotmail.go'
s = io.open(p, encoding='utf-8').read()
old = '\t\t\t\t\t\tcands := linkPattern.FindAllString(msg.BodyPreview+" "+rawBody, -1)'
new = old + '\n\t\t\t\t\t\tfor _, lk := range cands {\n\t\t\t\t\t\t\tfmt.Printf("[cursor-mail] %s | %s | %s\\n", msg.ReceivedDateTime, msg.Subject, lk)\n\t\t\t\t\t\t}'
assert old in s
io.open(p, 'w', encoding='utf-8', newline='').write(s.replace(old, new, 1))
print('ok')
