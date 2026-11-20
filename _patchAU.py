import io
p = r'C:\Users\dhl\Desktop\m3\go-protocol\cursor_exec.go'
s = io.open(p, encoding='utf-8').read()
anchor = '\t\t// Cookie 提示条：先关掉'
new = '\t\t// Cursor 的一次性引导页：点 Continue 过去。\n\t\tif strings.Contains(st.Text, "Before you Start Building") {\n\t\t\tif ok, _ := browser.clickAtReal(ctx, "Continue"); ok {\n\t\t\t\tif onStep != nil {\n\t\t\t\t\tonStep("完成 Cursor 引导页")\n\t\t\t\t}\n\t\t\t\tcursorDebug("onboarding continue clicked")\n\t\t\t\ttime.Sleep(3 * time.Second)\n\t\t\t\tcontinue\n\t\t\t}\n\t\t}\n\n'
assert anchor in s
io.open(p, 'w', encoding='utf-8', newline='').write(s.replace(anchor, new + anchor, 1))
print('ok')
