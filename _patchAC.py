import io
p = r'C:\Users\dhl\Desktop\m3\go-protocol\cursor_cli.go'
s = io.open(p, encoding='utf-8').read()
i = s.index('\tfetchCode := func(')
j = s.index('\tvar res *CursorResult')
block = s[i:j]
s = s[:i] + s[j:]
anchor = '\tmailClient := &http.Client{Timeout: 30 * time.Second}\n'
s = s.replace(anchor, anchor + block, 1)
io.open(p, 'w', encoding='utf-8', newline='').write(s)
print('ok')
