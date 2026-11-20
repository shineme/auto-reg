import io
p = r'C:\Users\dhl\Desktop\m3\go-protocol\cursor_flow.go'
s = io.open(p, encoding='utf-8').read()
code = """
// openConfirmTab 在同一浏览器新开标签打开确认链接，并点下页面上的 Confirm。
func (b *cursorBrowser) openConfirmTab(ctx context.Context, url string) (bool, error) {
	res, err := b.browserSend(ctx, "Target.createTarget", map[string]any{"url": url}, "")
	if err != nil {
		return false, err
	}
	result, _ := res["result"].(map[string]any)
	tid, _ := result["targetId"].(string)
	if tid == "" {
		return false, fmt.Errorf("新标签页创建失败")
	}
	ares, err := b.browserSend(ctx, "Target.attachToTarget", map[string]any{"targetId": tid, "flatten": true}, "")
	if err != nil {
		return false, err
	}
	aresult, _ := ares["result"].(map[string]any)
	sid, _ := aresult["sessionId"].(string)
	if sid == "" {
		return false, fmt.Errorf("无法附加到新标签页")
	}
	expr := "(function(){var bs=document.querySelectorAll('button');for(var i=0;i<bs.length;i++){var t=(bs[i].innerText||'').trim().toLowerCase();if(t==='confirm'||t==='确认'){bs[i].click();return true;}}return false;})()"
	for i := 0; i < 12; i++ {
		time.Sleep(1500 * time.Millisecond)
		out, err := b.browserSend(ctx, "Runtime.evaluate", map[string]any{"expression": expr, "returnByValue": true}, sid)
		if err != nil {
			continue
		}
		r2, _ := out["result"].(map[string]any)
		val, _ := r2["result"].(map[string]any)
		if v, _ := val["value"].(bool); v {
			return true, nil
		}
	}
	return false, nil
}
"""
io.open(p, 'w', encoding='utf-8', newline='').write(s + code)
print('ok')
