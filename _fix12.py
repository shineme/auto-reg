import io
p = r"C:\Users\dhl\Desktop\m3\go-protocol\main.go"
s = io.open(p, encoding="utf-8").read()
old = '\tcursorKey := flag.String("cursor-key", "", "panel key for cursor test")\n'
new = old + '\tcursorEmail := flag.String("cursor-email", "", "use a specific hotmail account for the cursor test")\n'
assert old in s
s = s.replace(old, new)
s = s.replace("runCLITestCursor(*cursorPanel, *cursorKey)", "runCLITestCursor(*cursorPanel, *cursorKey, *cursorEmail)")
io.open(p, "w", encoding="utf-8").write(s)
print("ok")
