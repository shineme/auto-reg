import io
p = r'C:\\Users\\dhl\\Desktop\\m3\\go-protocol\\cursor_app.go'
s = io.open(p, encoding='utf-8').read()
s = s.replace('Running: true, UpdatedAt: time.Now(),', 'Running: true, UpdatedAt: time.Now().Format("15:04:05"),', 1)
io.open(p,'w',encoding='utf-8',newline='').write(s)
print('ok')
