import io
p = r'C:\Users\dhl\Desktop\m3\_cdpdiag.py'
s = io.open(p, encoding='utf-8').read()
s = s.replace('"http://127.0.0.1:%d/json/list" % port', '"http://127.0.0.1:%s/json/list" % port')
io.open(p, 'w', encoding='utf-8', newline='').write(s)
print('ok')
