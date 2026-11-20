import io
p = r'C:\Users\dhl\Desktop\m3\go-protocol\cursor_cli.go'
s = io.open(p, encoding='utf-8').read()
old = 'return PollHotmailLink(ctx, mailClient, acc, notBefore, 3*time.Minute, "cursor")'
new = 'return PollHotmailLink(ctx, mailClient, acc, notBefore, 3*time.Minute, "cursor-confirm")'
assert old in s
io.open(p, 'w', encoding='utf-8', newline='').write(s.replace(old, new, 1))
print('ok')
