import io
p = r'C:\Users\dhl\Desktop\m3\go-protocol\cursor.go'
s = io.open(p, encoding='utf-8').read()
old = '\t// Password 新建 Cursor 账号时设置的密码，留空则用随机强密码。\n\tPassword string\n}'
new = '\t// Password 新建 Cursor 账号时设置的密码，留空则用随机强密码。\n\tPassword string\n\t// FetchLink 取回邮箱里的确认链接（Cursor 的“确认变更账号”邮件）。\n\tFetchLink func(ctx context.Context, notBefore time.Time) (string, error)\n}'
assert old in s
io.open(p, 'w', encoding='utf-8', newline='').write(s.replace(old, new, 1))
print('ok')
