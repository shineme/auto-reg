import io
p = r"C:\Users\dhl\Desktop\m3\go-protocol\cursor_cli.go"
s = io.open(p, encoding="utf-8").read()
old = "func runCLITestCursor(panelBase, panelKey string) {"
new = "func runCLITestCursor(panelBase, panelKey, email string) {"
assert old in s
s = s.replace(old, new)
old2 = "\tstore := NewHotmailPoolStore(poolPath)\n\tacc, idx, err := store.Claim()"
new2 = """\tstore := NewHotmailPoolStore(poolPath)
\tvar acc *HotmailAccount
\tvar idx int
\tif email != "" {
\t\tpool, perr := LoadHotmailPool(poolPath)
\t\tif perr != nil {
\t\t\tfmt.Printf("[cursor] 读取号池失败: %v\\n", perr)
\t\t\treturn
\t\t}
\t\tfor i := range pool.Accounts {
\t\t\tif strings.EqualFold(pool.Accounts[i].Email, email) {
\t\t\t\tacc = &pool.Accounts[i]
\t\t\t\tidx = i
\t\t\t\tbreak
\t\t\t}
\t\t}
\t\tif acc == nil {
\t\t\tfmt.Printf("[cursor] 号池里没有 %s\\n", email)
\t\t\treturn
\t\t}
\t} else {
\t\tvar cerr error
\t\tacc, idx, cerr = store.Claim()
\t\terr = cerr
\t}"""
assert old2 in s
s = s.replace(old2, new2)
s = s.replace('import (\n\t"context"\n\t"encoding/json"\n\t"fmt"\n\t"net/http"\n\t"os"\n\t"path/filepath"\n\t"time"\n)', 'import (\n\t"context"\n\t"encoding/json"\n\t"fmt"\n\t"net/http"\n\t"os"\n\t"path/filepath"\n\t"strings"\n\t"time"\n)')
io.open(p, "w", encoding="utf-8").write(s)
print("ok")
