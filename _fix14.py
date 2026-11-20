import io
p = r"C:\Users\dhl\Desktop\m3\go-protocol\cursor_exec.go"
s = io.open(p, encoding="utf-8").read()
old = 'if strings.Contains(st.Host, "authenticator") && !st.HasCode {'
new = 'if strings.Contains(st.Host, "authenticator") && !st.HasCode && switchTries < 3 {'
assert old in s
s = s.replace(old, new)
old2 = 'cursorDebug("switched login mode: %s", clicked)'
new2 = 'switchTries++\n\t\t\t\tcursorDebug("switched login mode: %s", clicked)'
assert old2 in s
s = s.replace(old2, new2)
old3 = "\tpasswordTries := 0\n"
new3 = "\tpasswordTries := 0\n\tswitchTries := 0\n"
assert old3 in s
s = s.replace(old3, new3)
io.open(p, "w", encoding="utf-8").write(s)
print("ok")
