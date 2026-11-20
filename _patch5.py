import io, sys
p = r'C:\Users\dhl\Desktop\m3\go-protocol\cursor_flow.go'
s = io.open(p, encoding='utf-8').read()
a = s.index('// clickTurnstile')
b = s.index('// clickSel ')
s2 = s[:a] + s[b:]
io.open(p, 'w', encoding='utf-8', newline='').write(s2)
print('removed turnstile clicker')
