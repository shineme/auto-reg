import io
p = r'C:\Users\dhl\Desktop\m3\go-protocol\cursor_exec.go'
s = io.open(p, encoding='utf-8').read()
old = '\t\tif strings.Contains(st.Text, "Before you approve") || strings.Contains(st.Text, "asking to sign in with this account") {'
new = '\t\tif strings.Contains(strings.ToLower(st.URL), "logindeepcontrol") {'
assert old in s
io.open(p, 'w', encoding='utf-8', newline='').write(s.replace(old, new, 1))
print('ok')
