import io
p = r'C:\Users\dhl\Desktop\m3\go-protocol\mail_hotmail.go'
s = io.open(p, encoding='utf-8').read()
code = '''
// linkPattern 抓邮件正文里的 http 链接。
var linkPattern = regexp.MustCompile(`https?://[^\\s"'<>)\\\\]+`)

// PollHotmailLink 轮询邮箱，取回最新一封目标邮件里的链接（Cursor 的确认信用）。
func PollHotmailLink(ctx context.Context, client *http.Client, acc *HotmailAccount, notBefore time.Time, timeout time.Duration, kind string) (string, error) {
	accessToken, err := RefreshHotmailToken(ctx, client, acc)
	if err != nil {
		return "", err
	}
	deadline := time.Now().Add(timeout)
	notBeforeUTC := notBefore.UTC().Add(-2 * time.Minute)
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, hotmailGraphAPI+"/me/messages?$top=10&$select=subject,bodyPreview,body,receivedDateTime,from&$orderby=receivedDateTime%20desc", nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("Authorization", "Bearer "+accessToken)
		resp, err := client.Do(req)
		if err == nil {
			raw, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode == 401 {
				if tok, rerr := RefreshHotmailToken(ctx, client, acc); rerr == nil {
					accessToken = tok
				}
			} else if resp.StatusCode == 200 {
				var list struct {
					Value []struct {
						Subject          string `json:"subject"`
						BodyPreview      string `json:"bodyPreview"`
						ReceivedDateTime string `json:"receivedDateTime"`
						From             struct {
							EmailAddress struct {
								Address string `json:"address"`
							} `json:"emailAddress"`
						} `json:"from"`
						Body struct {
							Content string `json:"content"`
						} `json:"body"`
					} `json:"value"`
				}
				if json.Unmarshal(raw, &list) == nil {
					for _, msg := range list.Value {
						if !notBeforeUTC.IsZero() && msg.ReceivedDateTime != "" {
							received, perr := time.Parse(time.RFC3339, msg.ReceivedDateTime)
							if perr == nil && received.Before(notBeforeUTC) {
								continue
							}
						}
						sender := strings.ToLower(msg.From.EmailAddress.Address)
						subject := strings.ToLower(msg.Subject)
						if !mailKindMatches(kind, sender, subject) {
							continue
						}
						body := cleanMailBody(msg.Body.Content)
						body = strings.ReplaceAll(body, "&amp;", "&")
						for _, link := range linkPattern.FindAllString(msg.BodyPreview+" "+body, -1) {
							l := strings.ToLower(link)
							if strings.Contains(l, "cursor.com") {
								return link, nil
							}
						}
					}
				}
			}
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(4 * time.Second):
		}
	}
	return "", fmt.Errorf("Hotmail Graph 轮询超时，未收到 %s 的确认链接", acc.Email)
}
'''
io.open(p, 'w', encoding='utf-8', newline='').write(s + code)
print('ok')
