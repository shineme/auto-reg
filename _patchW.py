import io
p = r'C:\Users\dhl\Desktop\m3\go-protocol\cursor_exec.go'
s = io.open(p, encoding='utf-8').read()
old = '\tif onStep != nil {\n\t\tonStep("申请面板授权")\n\t}'
new = '\tcursorDebug("signIn id=%s url=%s", signIn.ID, signIn.URL)\n\tif onStep != nil {\n\t\tonStep("申请面板授权")\n\t}'
assert old in s
io.open(p, 'w', encoding='utf-8', newline='').write(s.replace(old, new, 1))
print('ok')
