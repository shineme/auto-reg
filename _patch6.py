import io, sys
Q = chr(34)
p = r'C:\Users\dhl\Desktop\m3\go-protocol\cursor_exec.go'
s = io.open(p, encoding='utf-8').read()
a = s.index('// 人机验证')
b = s.index('// 密码页')
new = ('\t\t// 人机验证：程序不代过验证，由用户在浏览器窗口自行点击完成。\n'
 '\t\tif strings.Contains(st.Text, ' + Q + 'sure you are human' + Q + ') || strings.Contains(st.Text, ' + Q + 'verify the user is human' + Q + ') {\n'
 '\t\t\tif !humanHinted {\n'
 '\t\t\t\thumanHinted = true\n'
 '\t\t\t\tif onStep != nil {\n'
 '\t\t\t\t\tonStep(' + Q + '请在浏览器窗口手动完成人机验证，完成后自动继续' + Q + ')\n'
 '\t\t\t\t}\n'
 '\t\t\t\tfmt.Println(' + Q + '[cursor] 请在 Chrome 窗口点击人机验证（窗口已置于屏幕左上角）' + Q + ')\n'
 '\t\t\t}\n'
 '\t\t\ttime.Sleep(2 * time.Second)\n'
 '\t\t\tcontinue\n'
 '\t\t}\n\n')
io.open(p, 'w', encoding='utf-8', newline='').write(s[:a] + new + s[b:])
print('ok')
