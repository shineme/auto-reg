import io
p = r'C:\Users\dhl\Desktop\m3\go-protocol\cursor.go'
s = io.open(p, encoding='utf-8').read()
anchor = '// ============================ 浏览器驱动 ============================'
code = """// CursorSignInState 读一次 sign-in 记录的状态（waiting/done/error）。
func (p *PanelClient) CursorSignInState(ctx context.Context, signInID string) (string, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/api/signin/"+url.PathEscape(signInID), nil)
	if err != nil {
		return "", "", err
	}
	resp, err := p.http.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", "", fmt.Errorf("GET /api/signin/%s -> %d", signInID, resp.StatusCode)
	}
	var out SignInPollResp
	if err := json.Unmarshal(body, &out); err != nil {
		return "", "", err
	}
	return out.State, out.User, nil
}

"""
assert anchor in s
io.open(p, 'w', encoding='utf-8', newline='').write(s.replace(anchor, code + anchor, 1))
print('ok')
