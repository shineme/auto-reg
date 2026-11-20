import io
p = r'C:\Users\dhl\Desktop\m3\go-protocol\cursor_exec.go'
s = io.open(p, encoding='utf-8').read()
old = '\t\tif st.HasPassword && passwordTries < 3 {'
new = '\t\tif st.HasPassword && passwordTries < 3 && !strings.Contains(st.URL, "/magic-code") {'
assert old in s
io.open(p, 'w', encoding='utf-8', newline='').write(s.replace(old, new, 1))
print('ok')
