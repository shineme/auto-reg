import io
p = r'C:\Users\dhl\Desktop\m3\go-protocol\cursor_flow.go'
s = io.open(p, encoding='utf-8').read()
old = "        return t && t.indexOf('cancel') < 0 && t.indexOf('取消') < 0;"
new = "        return t && t.indexOf('cancel') < 0 && t.indexOf('取消') < 0 && t.indexOf('resend') < 0 && t.indexOf('重发') < 0;"
assert old in s
s = s.replace(old, new, 1)
s = s.replace('import (\n\t"context"\n\t"encoding/json"\n\t"fmt"\n\t"os"\n\t"strings"\n)', 'import (\n\t"context"\n\t"encoding/json"\n\t"fmt"\n\t"os"\n\t"strings"\n\t"time"\n)', 1)
io.open(p, 'w', encoding='utf-8', newline='').write(s)
print('ok')
