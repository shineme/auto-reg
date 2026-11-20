import io
p = r'C:\Users\dhl\Desktop\m3\go-protocol\mail_hotmail.go'
s = io.open(p, encoding='utf-8').read()
old = """						body := cleanMailBody(msg.Body.Content)
						body = strings.ReplaceAll(body, "&amp;", "&")
						for _, link := range linkPattern.FindAllString(msg.BodyPreview+" "+body, -1) {"""
new = """						rawBody := strings.ReplaceAll(msg.Body.Content, "&amp;", "&")
						rawBody = strings.ReplaceAll(rawBody, "&#38;", "&")
						for _, link := range linkPattern.FindAllString(msg.BodyPreview+" "+rawBody, -1) {"""
assert old in s
io.open(p, 'w', encoding='utf-8', newline='').write(s.replace(old, new, 1))
print('ok')
