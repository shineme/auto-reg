import io
p = r"C:\Users\dhl\Desktop\m3\go-protocol\cursor_flow.go"
s = io.open(p, encoding="utf-8").read()
s = s.replace('\t"strings"\n\t"time"\n', '\t"strings"\n')
io.open(p, "w", encoding="utf-8").write(s)
print("ok")
