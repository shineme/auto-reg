import io
p = r'C:\\Users\\dhl\\Desktop\\m3\\go-protocol\\cursor_app.go'
s = io.open(p, encoding='utf-8').read()
s = s.replace('import (\n\t"context"\n\t"fmt"\n\t"path/filepath"\n\t"sync"\n\t"time"\n)', 'import (\n\t"context"\n\t"fmt"\n\t"net/http"\n\t"path/filepath"\n\t"sync"\n\t"time"\n)', 1)
io.open(p,'w',encoding='utf-8',newline='').write(s)
print('ok')
