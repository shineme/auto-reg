import io
p = r'C:\Users\dhl\Desktop\m3\go-protocol\cursor_flow.go'
s = io.open(p, encoding='utf-8').read()
old = '\t"strings"\n\t"time"\n)'
new = '\t"strings"\n\t"time"\n\n\t"github.com/gorilla/websocket"\n)'
assert old in s
io.open(p, 'w', encoding='utf-8', newline='').write(s.replace(old, new, 1))
print('ok')
