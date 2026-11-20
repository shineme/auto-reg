import io
p = r'C:\Users\dhl\Desktop\m3\go-protocol\cursor_exec.go'
s = io.open(p, encoding='utf-8').read()
s = s.replace('\thumanHinted := false\n', '\thumanHinted := false\n\tapproveTries := 0\n', 1)
old = '\t\tif strings.Contains(strings.ToLower(st.URL), "logindeepcontrol") {'
new = '\t\tif strings.Contains(strings.ToLower(st.URL), "logindeepcontrol") && approveTries < 5 {'
assert old in s
s = s.replace(old, new, 1)
old2 = '\t\t\t\tcursorDebug("approved sign-in request")'
new2 = '\t\t\t\tapproveTries++\n\t\t\t\tcursorDebug("approved sign-in request")'
assert old2 in s
s = s.replace(old2, new2, 1)
io.open(p, 'w', encoding='utf-8', newline='').write(s)
print('ok')
