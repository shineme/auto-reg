import io
p = r"C:\Users\dhl\Desktop\m3\go-protocol\mail_hotmail.go"
s = io.open(p, encoding="utf-8").read()

old_sig = """func PollHotmailCode(ctx context.Context, client *http.Client, acc *HotmailAccount, notBefore time.Time, timeout time.Duration) (string, error) {
	accessToken, err := RefreshHotmailToken(ctx, client, acc)"""
new_sig = """func PollHotmailCode(ctx context.Context, client *http.Client, acc *HotmailAccount, notBefore time.Time, timeout time.Duration) (string, error) {
	return PollHotmailCodeFor(ctx, client, acc, notBefore, timeout, "minimax")
}

// PollHotmailCodeFor 按邮件来源过滤验证码：kind 取 minimax 或 cursor。
func PollHotmailCodeFor(ctx context.Context, client *http.Client, acc *HotmailAccount, notBefore time.Time, timeout time.Duration, kind string) (string, error) {
	accessToken, err := RefreshHotmailToken(ctx, client, acc)"""
assert old_sig in s
s = s.replace(old_sig, new_sig)

old_filter = """						sender := strings.ToLower(msg.From.EmailAddress.Address)
						if sender == "" || strings.Contains(sender, "minimax") || strings.Contains(sender, "hailuo") {
							return codes[0], nil
						}"""
new_filter = """						sender := strings.ToLower(msg.From.EmailAddress.Address)
						subject := strings.ToLower(msg.Subject)
						if mailKindMatches(kind, sender, subject) {
							return codes[0], nil
						}"""
assert old_filter in s
s = s.replace(old_filter, new_filter)

helper = """
// mailKindMatches 判断一封邮件是否是目标 provider 发的验证码邮件。
func mailKindMatches(kind, sender, subject string) bool {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "cursor":
		return strings.Contains(sender, "cursor") || strings.Contains(subject, "cursor") || strings.Contains(subject, "登录 cursor")
	default:
		return sender == "" || strings.Contains(sender, "minimax") || strings.Contains(sender, "hailuo")
	}
}
"""
s = s.rstrip() + "\n" + helper
io.open(p, "w", encoding="utf-8").write(s)
print("mail_hotmail.go patched")
