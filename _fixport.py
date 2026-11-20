import io
s = io.open(r'C:\Users\dhl\Desktop\m3\_els.py', encoding='utf-8').read()
import re
s = re.sub(r'port = \d+', 'port = PORTNUM', s)
io.open(r'C:\Users\dhl\Desktop\m3\_els.py','w',encoding='utf-8',newline='').write(s)
