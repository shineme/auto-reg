package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type MailScanMessage struct {
	Subject          string `json:"subject"`
	From             string `json:"from"`
	ReceivedDateTime string `json:"receivedAt"`
}

type MailScanAccountResult struct {
	Email             string            `json:"email"`
	PoolStatus        string            `json:"poolStatus"`
	TokenDead         bool              `json:"tokenDead"`
	Error             string            `json:"error,omitempty"`
	CursorRegistered  bool              `json:"cursorRegistered"`
	CursorFirstSeen   string            `json:"cursorFirstSeen,omitempty"`
	CursorLastSeen    string            `json:"cursorLastSeen,omitempty"`
	CursorMailCount   int               `json:"cursorMailCount"`
	MiniMaxMailCount  int               `json:"minimaxMailCount"`
	LastInboxActivity string            `json:"lastInboxActivity,omitempty"`
	LatestMessages    []MailScanMessage `json:"latestMessages,omitempty"`
}

type MailScanReport struct {
	ScannedAt        string                  `json:"scannedAt"`
	PoolPath         string                  `json:"poolPath"`
	Total            int                     `json:"total"`
	Scanned          int                     `json:"scanned"`
	TokenDead        int                     `json:"tokenDead"`
	CursorRegistered int                     `json:"cursorRegistered"`
	WithMiniMaxMails int                     `json:"withMiniMaxMails"`
	ReportPath       string                  `json:"reportPath"`
	Accounts         []MailScanAccountResult `json:"accounts"`
}

// MailInboxItem 收件箱里的一条邮件（列表视图，不含正文）。
type MailInboxItem struct {
	ID         string `json:"id"`
	Subject    string `json:"subject"`
	From       string `json:"from"`
	ReceivedAt string `json:"receivedAt"`
	Preview    string `json:"preview"`
	IsRead     bool   `json:"isRead"`
	Folder     string `json:"folder"`
}

// MailInboxView 某个邮箱的收件箱 + 垃圾邮箱实时列表。
type MailInboxView struct {
	Email  string          `json:"email"`
	Items  []MailInboxItem `json:"items"`
	TokenOK bool           `json:"tokenOk"`
	Error  string          `json:"error,omitempty"`
}

// MailMessageFull 单封邮件的完整内容。
type MailMessageFull struct {
	Email      string `json:"email"`
	Subject    string `json:"subject"`
	From       string `json:"from"`
	To         string `json:"to"`
	ReceivedAt string `json:"receivedAt"`
	BodyHTML   string `json:"bodyHtml"`
	BodyText   string `json:"bodyText"`
}

func FindPoolAccount(poolPath, email string) (*HotmailAccount, error) {
	pool, err := LoadHotmailPool(poolPath)
	if err != nil {
		return nil, err
	}
	for i := range pool.Accounts {
		if strings.EqualFold(pool.Accounts[i].Email, email) {
			return &pool.Accounts[i], nil
		}
	}
	return nil, fmt.Errorf("号池里找不到邮箱 %s", email)
}

func newMailScanClient(viaProxy bool, proxyURL string) (*http.Client, error) {
	if viaProxy {
		d, err := NewProxyDialer(proxyURL)
		if err != nil {
			return nil, err
		}
		return d.HTTPClient(30 * time.Second), nil
	}
	return &http.Client{Timeout: 30 * time.Second}, nil
}

func refreshPoolToken(ctx context.Context, client *http.Client, poolPath, email string) (*HotmailAccount, string, error) {
	acc, err := FindPoolAccount(poolPath, email)
	if err != nil {
		return nil, "", err
	}
	token, err := RefreshHotmailToken(ctx, client, acc)
	if err != nil {
		return acc, "", err
	}
	return acc, token, nil
}

func fetchGraphItems(ctx context.Context, client *http.Client, acc *HotmailAccount, token, folder string, top int) ([]MailInboxItem, error) {
	uri := "/me/messages"
	if folder == "junkemail" {
		uri = "/me/mailFolders/junkemail/messages"
	}
	reqURL := hotmailGraphAPI + uri + "?$top=" + fmt.Sprintf("%d", top) +
		"&$select=id,subject,from,receivedDateTime,bodyPreview,isRead&$orderby=receivedDateTime%20desc"
	for attempt := 0; attempt < 2; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		raw, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode == 401 && attempt == 0 {
			if token, err = RefreshHotmailToken(ctx, client, acc); err == nil {
				continue
			}
		}
		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("graph %s HTTP %d: %s", folder, resp.StatusCode, strings.TrimSpace(string(raw)))
		}
		var list struct {
			Value []struct {
				ID             string `json:"id"`
				Subject        string `json:"subject"`
				ReceivedAt     string `json:"receivedDateTime"`
				Preview        string `json:"bodyPreview"`
				IsRead       bool   `json:"isRead"`
				From           struct {
					EmailAddress struct {
						Address string `json:"address"`
					} `json:"emailAddress"`
				} `json:"from"`
			} `json:"value"`
		}
		if err := json.Unmarshal(raw, &list); err != nil {
			return nil, err
		}
		out := make([]MailInboxItem, 0, len(list.Value))
		for _, m := range list.Value {
			out = append(out, MailInboxItem{
				ID:         m.ID,
				Subject:    m.Subject,
				From:       m.From.EmailAddress.Address,
				ReceivedAt: m.ReceivedAt,
				Preview:    m.Preview,
				IsRead:     m.IsRead,
				Folder:     folder,
			})
		}
		return out, nil
	}
	return nil, fmt.Errorf("graph %s 令牌刷新失败", folder)
}

// ListMailInbox 实时拉取某个邮箱的收件箱（前 50 封）+ 垃圾邮箱（前 20 封）。
// 只读取号池，不回写，不会影响正在跑的注册进程。
func ListMailInbox(ctx context.Context, poolPath, email string, viaProxy bool, proxyURL string) (MailInboxView, error) {
	client, err := newMailScanClient(viaProxy, proxyURL)
	if err != nil {
		return MailInboxView{}, err
	}
	acc, token, err := refreshPoolToken(ctx, client, poolPath, email)
	if err != nil {
		view := MailInboxView{Email: email, Items: []MailInboxItem{}, Error: err.Error()}
		return view, nil
	}
	inbox, err1 := fetchGraphItems(ctx, client, acc, token, "inbox", 50)
	junk, err2 := fetchGraphItems(ctx, client, acc, token, "junkemail", 20)
	items := append(inbox, junk...)
	if err1 != nil || err2 != nil {
		msg := err1.Error()
		if err2 != nil {
			msg += "；垃圾箱: " + err2.Error()
		}
		return MailInboxView{Email: email, Items: items, TokenOK: true, Error: msg}, nil
	}
	return MailInboxView{Email: email, Items: items, TokenOK: true}, nil
}

// GetMailMessage 拉取单封邮件的完整正文（HTML + 纯文本）。
func GetMailMessage(ctx context.Context, poolPath, email, messageID string, viaProxy bool, proxyURL string) (MailMessageFull, error) {
	client, err := newMailScanClient(viaProxy, proxyURL)
	if err != nil {
		return MailMessageFull{}, err
	}
	acc, token, err := refreshPoolToken(ctx, client, poolPath, email)
	if err != nil {
		return MailMessageFull{}, err
	}

	reqURL := hotmailGraphAPI + "/me/messages/" + url.PathEscape(messageID) + "?$select=subject,from,toRecipients,receivedDateTime,body"
	var out MailMessageFull
	for attempt := 0; attempt < 2; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
		if err != nil {
			return out, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := client.Do(req)
		if err != nil {
			return out, err
		}
		raw, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode == 401 && attempt == 0 {
			if token, err = RefreshHotmailToken(ctx, client, acc); err == nil {
				continue
			}
		}
		if resp.StatusCode != 200 {
			return out, fmt.Errorf("graph 取信正文 HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
		}
		var msg struct {
			Subject    string `json:"subject"`
			ReceivedAt string `json:"receivedDateTime"`
			From       struct {
				EmailAddress struct {
					Address string `json:"address"`
				} `json:"emailAddress"`
			} `json:"from"`
			To   []struct {
				EmailAddress struct {
					Address string `json:"address"`
				} `json:"emailAddress"`
			} `json:"toRecipients"`
			Body struct {
				ContentType string `json:"contentType"`
				Content     string `json:"content"`
			} `json:"body"`
		}
		if err := json.Unmarshal(raw, &msg); err != nil {
			return out, err
		}
		out.Email = email
		out.Subject = msg.Subject
		out.From = msg.From.EmailAddress.Address
		out.ReceivedAt = msg.ReceivedAt
		for i, t := range msg.To {
			if i > 0 {
				out.To += ", "
			}
			out.To += t.EmailAddress.Address
		}
		body := msg.Body.Content
		if strings.EqualFold(msg.Body.ContentType, "text/html") {
			out.BodyHTML = body
			out.BodyText = cleanMailBody(body)
		} else {
			out.BodyText = body
			out.BodyHTML = "<pre>" + escapeHTML(body) + "</pre>"
		}
		return out, nil
	}
	return out, fmt.Errorf("graph 取信正文失败：令牌刷新两次均未成功")
}

func escapeHTML(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
	return r.Replace(s)
}

// MailScanEmails 返回号池全部邮箱 + （若有报告）检出 Cursor 的邮箱清单，供 GUI 自动补全。
func MailScanEmails(poolPath string) (all []string, cursor []string, err error) {
	pool, err := LoadHotmailPool(poolPath)
	if err != nil {
		return nil, nil, err
	}
	for _, a := range pool.Accounts {
		all = append(all, a.Email)
	}
	if data, rerr := os.ReadFile(filepath.Join(filepath.Dir(poolPath), "mailscan-report.json")); rerr == nil {
		var rep MailScanReport
		if json.Unmarshal(data, &rep) == nil {
			for _, a := range rep.Accounts {
				if a.CursorRegistered {
					cursor = append(cursor, a.Email)
				}
			}
		}
	}
	return all, cursor, nil
}
func isCursorMail(from, subject string) bool {
	f := strings.ToLower(from)
	s := strings.ToLower(subject)
	if strings.Contains(f, "cursor.sh") || strings.Contains(f, "cursor.com") {
		return true
	}
	return strings.Contains(s, "cursor")
}

func isMiniMaxMail(from, subject string) bool {
	combined := strings.ToLower(from + " " + subject)
	return strings.Contains(combined, "minimax") || strings.Contains(combined, "hailuo")
}

// scanOneAccount 刷新令牌并拉取收件箱+垃圾邮箱的最新信件，返回该邮箱的判定结果。
func scanOneAccount(ctx context.Context, client *http.Client, acc *HotmailAccount, res *MailScanAccountResult) {
	token, err := RefreshHotmailToken(ctx, client, acc)
	if err != nil {
		res.TokenDead = true
		res.Error = err.Error()
		return
	}
	msgs, err := fetchGraphFolder(ctx, client, acc, token, "inbox", 20)
	if err != nil {
		res.Error = err.Error()
		return
	}
	junk, _ := fetchGraphFolder(ctx, client, acc, token, "junkemail", 10)
	all := append(msgs, junk...)
	for _, m := range all {
		if isCursorMail(m.From, m.Subject) {
			res.CursorMailCount++
			if res.CursorFirstSeen == "" || m.ReceivedDateTime < res.CursorFirstSeen {
				res.CursorFirstSeen = m.ReceivedDateTime
			}
			if m.ReceivedDateTime > res.CursorLastSeen {
				res.CursorLastSeen = m.ReceivedDateTime
			}
		}
		if isMiniMaxMail(m.From, m.Subject) {
			res.MiniMaxMailCount++
		}
	}
	res.CursorRegistered = res.CursorMailCount > 0
	for _, m := range all {
		if m.ReceivedDateTime > res.LastInboxActivity {
			res.LastInboxActivity = m.ReceivedDateTime
		}
	}
	res.LatestMessages = all
}

func fetchGraphFolder(ctx context.Context, client *http.Client, acc *HotmailAccount, token, folder string, top int) ([]MailScanMessage, error) {
	uri := "/me/messages"
	if folder == "junkemail" {
		uri = "/me/mailFolders/junkemail/messages"
	}
	reqURL := hotmailGraphAPI + uri + "?$top=" + fmt.Sprintf("%d", top) + "&$select=subject,from,receivedDateTime&$orderby=receivedDateTime%20desc"
	for attempt := 0; attempt < 2; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		raw, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode == 401 && attempt == 0 {
			if token, err = RefreshHotmailToken(ctx, client, acc); err == nil {
				continue
			}
		}
		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("graph %s HTTP %d: %s", folder, resp.StatusCode, strings.TrimSpace(string(raw)))
		}
		var list struct {
			Value []struct {
				Subject          string `json:"subject"`
				ReceivedDateTime string `json:"receivedDateTime"`
				From             struct {
					EmailAddress struct {
						Address string `json:"address"`
					} `json:"emailAddress"`
				} `json:"from"`
			} `json:"value"`
		}
		if err := json.Unmarshal(raw, &list); err != nil {
			return nil, err
		}
		out := make([]MailScanMessage, 0, len(list.Value))
		for _, m := range list.Value {
			out = append(out, MailScanMessage{
				Subject:          m.Subject,
				From:             m.From.EmailAddress.Address,
				ReceivedDateTime: m.ReceivedDateTime,
			})
		}
		return out, nil
	}
	return nil, fmt.Errorf("graph %s 令牌刷新失败", folder)
}

// ScanHotmailInboxes 多线程扫描号池邮箱：判定 Cursor 注册痕迹 + 拉取最新收件箱。
// 只写独立报告文件（池子目录下的 mailscan-report.json / .csv），不改动池子状态。
// limit > 0 时只扫描前 limit 个账号（用于快速测试）。
func ScanHotmailInboxes(ctx context.Context, poolPath string, workers, limit int, viaProxy bool, proxyURL string, onLog func(string)) (*MailScanReport, error) {
	pool, err := LoadHotmailPool(poolPath)
	if err != nil {
		return nil, err
	}
	if workers <= 0 {
		workers = 16
	}
	var client *http.Client
	if viaProxy {
		d, perr := NewProxyDialer(proxyURL)
		if perr != nil {
			return nil, perr
		}
		client = d.HTTPClient(30 * time.Second)
	} else {
		client = &http.Client{Timeout: 30 * time.Second}
	}

	var indices []int
	for i, a := range pool.Accounts {
		if a.Status == "skipped" || a.Status == "error" {
			continue
		}
		indices = append(indices, i)
		if limit > 0 && len(indices) >= limit {
			break
		}
	}

	report := &MailScanReport{
		ScannedAt: time.Now().Format(time.RFC3339),
		PoolPath:  poolPath,
		Total:     len(indices),
		Accounts:  []MailScanAccountResult{},
	}
	results := make([]MailScanAccountResult, len(indices))

	var mu sync.Mutex
	jobs := make(chan int, len(indices))
	for _, idx := range indices {
		jobs <- idx
	}
	close(jobs)

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				if ctx.Err() != nil {
					return
				}
				res := MailScanAccountResult{Email: pool.Accounts[idx].Email, PoolStatus: pool.Accounts[idx].Status}
				scanOneAccount(ctx, client, &pool.Accounts[idx], &res)
				mu.Lock()
				results[idx] = res
				report.Accounts = append(report.Accounts, res)
				if res.TokenDead {
					report.TokenDead++
				}
				if res.CursorRegistered {
					report.CursorRegistered++
				}
				if res.MiniMaxMailCount > 0 {
					report.WithMiniMaxMails++
				}
				report.Scanned++
				n := report.Scanned
				mu.Unlock()
				if onLog != nil && (n%50 == 0 || n == len(indices)) {
					onLog(fmt.Sprintf("[mailscan] %d/%d 已扫，Cursor 注册 %d，令牌失效 %d", n, len(indices), report.CursorRegistered, report.TokenDead))
				}
			}
		}()
	}
	wg.Wait()

	_ = results
	report.ReportPath = filepath.Join(filepath.Dir(poolPath), "mailscan-report.json")
	data, err := json.MarshalIndent(report, "", "  ")
	if err == nil {
		_ = os.WriteFile(report.ReportPath, data, 0o600)
	}
	csvPath := filepath.Join(filepath.Dir(poolPath), "mailscan-report.csv")
	if f, cerr := os.Create(csvPath); cerr == nil {
		w := csv.NewWriter(f)
		_ = w.Write([]string{"email", "poolStatus", "tokenDead", "cursorRegistered", "cursorFirstSeen", "cursorLastSeen", "cursorMails", "minimaxMails", "lastInboxActivity"})
		for _, a := range report.Accounts {
			_ = w.Write([]string{
				a.Email, a.PoolStatus,
				fmt.Sprintf("%v", a.TokenDead),
				fmt.Sprintf("%v", a.CursorRegistered),
				a.CursorFirstSeen, a.CursorLastSeen,
				fmt.Sprintf("%d", a.CursorMailCount),
				fmt.Sprintf("%d", a.MiniMaxMailCount),
				a.LastInboxActivity,
			})
		}
		w.Flush()
		_ = f.Close()
	}
	return report, nil
}

// PrintMailScanReport 打印人类可读的扫描结论。
func PrintMailScanReport(r *MailScanReport) {
	fmt.Printf("MAILSCAN_DONE scanned=%d total=%d tokenDead=%d cursorRegistered=%d minimaxMails=%d\n",
		r.Scanned, r.Total, r.TokenDead, r.CursorRegistered, r.WithMiniMaxMails)
	cursorAccounts := make([]MailScanAccountResult, 0)
	for _, a := range r.Accounts {
		if a.CursorRegistered {
			cursorAccounts = append(cursorAccounts, a)
		}
	}
	sort.Slice(cursorAccounts, func(i, j int) bool {
		return cursorAccounts[i].CursorFirstSeen < cursorAccounts[j].CursorFirstSeen
	})
	fmt.Printf("Cursor 注册过的邮箱 %d 个：\n", len(cursorAccounts))
	for i, a := range cursorAccounts {
		if i >= 50 {
			fmt.Printf("  ...（其余 %d 个见报告 %s）\n", len(cursorAccounts)-i-1, r.ReportPath)
			break
		}
		fmt.Printf("  %s  证据时间 %s ~ %s（%d 封）\n", a.Email, a.CursorFirstSeen, a.CursorLastSeen, a.CursorMailCount)
	}
}
