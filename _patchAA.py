import io
p = r'C:\Users\dhl\Desktop\m3\go-protocol\cursor_exec.go'
s = io.open(p, encoding='utf-8').read()
anchor = '\t\t// 面板 sign-in 记录变 done 就收工'
new = """\t\t// Cursor CLI 会发一封“确认变更账号”邮件，点信里的链接才算登录完成。
\t\tif strings.Contains(st.Text, "Confirm a change to your Cursor account") ||
\t\t\tstrings.Contains(st.Text, "Check your email on this device") ||
\t\t\tstrings.Contains(st.Text, "Waiting for you to confirm") {
\t\t\tif confirmLink == "" && f.FetchLink != nil && linkTries < 3 {
\t\t\t\tlinkTries++
\t\t\t\tif onStep != nil {
\t\t\t\t\tonStep("等待邮箱里的账号确认邮件")
\t\t\t\t}
\t\t\t\tlink, lerr := f.FetchLink(ctx, time.Now().Add(-10*time.Minute))
\t\t\t\tif lerr == nil && link != "" {
\t\t\t\t\tconfirmLink = link
\t\t\t\t} else {
\t\t\t\t\tcursorDebug("confirm link fetch failed: %v", lerr)
\t\t\t\t}
\t\t\t}
\t\t\tif confirmLink != "" {
\t\t\t\tif onStep != nil {
\t\t\t\t\tonStep("打开确认链接完成登录")
\t\t\t\t}
\t\t\t\tcursorDebug("open confirm link")
\t\t\t\tbrowser.send("Page.navigate", map[string]any{"url": confirmLink})
\t\t\t\tconfirmLink = ""
\t\t\t\ttime.Sleep(4 * time.Second)
\t\t\t\tcontinue
\t\t\t}
\t\t\ttime.Sleep(3 * time.Second)
\t\t\tcontinue
\t\t}

"""
assert anchor in s
s = s.replace(anchor, new + anchor, 1)
s = s.replace('\tapproveTries := 0\n', '\tapproveTries := 0\n\tlinkTries := 0\n\tconfirmLink := ""\n', 1)
io.open(p, 'w', encoding='utf-8', newline='').write(s)
print('ok')
