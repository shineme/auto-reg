import io
p = r"C:\Users\dhl\Desktop\m3\go-protocol\main.go"
s = io.open(p, encoding="utf-8").read()
old = '\tsupervised := flag.Bool("supervised", false, "supervised batch: auto-restart on failure until target reached or pool exhausted")\n'
new = old + '\ttestCursor := flag.Bool("test-cursor", false, "run one Cursor registration test with a hotmail account")\n\tcursorPanel := flag.String("cursor-panel", "", "panel base url for cursor test")\n\tcursorKey := flag.String("cursor-key", "", "panel key for cursor test")\n'
assert old in s
s = s.replace(old, new)
old2 = "\tif *supervised {\n"
new2 = "\tif *testCursor {\n\t\trunCLITestCursor(*cursorPanel, *cursorKey)\n\t\treturn\n\t}\n" + old2
assert old2 in s
s = s.replace(old2, new2)
io.open(p, "w", encoding="utf-8").write(s)
print("main.go patched")
