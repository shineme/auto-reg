import io
p = r'C:\Users\dhl\Desktop\m3\go-protocol\cursor_cli.go'
s = io.open(p, encoding='utf-8').read()
old = """	fetchCode := func(ctx context.Context, notBefore time.Time) (string, error) {
		return PollHotmailCodeFor(ctx, mailClient, acc, notBefore, 3*time.Minute, "cursor")
	}"""
new = """	fetchCode := func(ctx context.Context, notBefore time.Time) (string, error) {
		return PollHotmailCodeFor(ctx, mailClient, acc, notBefore, 3*time.Minute, "cursor")
	}
	fetchLink := func(ctx context.Context, notBefore time.Time) (string, error) {
		return PollHotmailLink(ctx, mailClient, acc, notBefore, 3*time.Minute, "cursor")
	}"""
assert old in s
s = s.replace(old, new, 1)
old2 = """		ProfileDir: filepath.Join(cursorTestDir(cfg), "chrome-profiles"),
	}"""
new2 = """		ProfileDir: filepath.Join(cursorTestDir(cfg), "chrome-profiles"),
		FetchLink:  fetchLink,
	}"""
assert old2 in s
s = s.replace(old2, new2, 1)
io.open(p, 'w', encoding='utf-8', newline='').write(s)
print('ok')
