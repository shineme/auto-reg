import io
p = r'C:\Users\dhl\Desktop\m3\go-protocol\cursor.go'
s = io.open(p, encoding='utf-8').read()
old = '\tb.conn = conn\n'
new = '\tb.conn = conn\n\tif vresp, verr := directHTTP.Get(fmt.Sprintf("http://127.0.0.1:%d/json/version", port)); verr == nil {\n\t\tvraw, _ := io.ReadAll(vresp.Body)\n\t\t_ = vresp.Body.Close()\n\t\tvar ver struct {\n\t\t\tWebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`\n\t\t}\n\t\tif json.Unmarshal(vraw, &ver) == nil && ver.WebSocketDebuggerURL != "" {\n\t\t\tif bconn, _, derr := websocket.DefaultDialer.DialContext(ctx, ver.WebSocketDebuggerURL, nil); derr == nil {\n\t\t\t\tb.bws = bconn\n\t\t\t}\n\t\t}\n\t}\n'
assert old in s
s = s.replace(old, new, 1)
old2 = '\t// profileDir 是本次运行用的临时 Chrome 配置目录，结束时删掉。\n\tprofileDir string\n}'
new2 = '\t// profileDir 是本次运行用的临时 Chrome 配置目录，结束时删掉。\n\tprofileDir string\n\t// bws 是浏览器级 CDP 连接，用来开新标签页。\n\tbws    *websocket.Conn\n\tbmsgID int\n}'
assert old2 in s
s = s.replace(old2, new2, 1)
old3 = '\tif b.conn != nil {\n\t\t_ = b.conn.Close()\n\t}'
new3 = '\tif b.bws != nil {\n\t\t_ = b.bws.Close()\n\t}\n\tif b.conn != nil {\n\t\t_ = b.conn.Close()\n\t}'
assert old3 in s
s = s.replace(old3, new3, 1)
io.open(p, 'w', encoding='utf-8', newline='').write(s)
print('ok')
