import io
p = r'C:\Users\dhl\Desktop\m3\go-protocol\cursor_flow.go'
s = io.open(p, encoding='utf-8').read()
old = "      var t = (el.innerText || el.value || el.textContent || '').trim();\n      if (t.toLowerCase() === want) { el.click(); return t; }"
new = "      var t = (el.innerText || el.getAttribute('aria-label') || el.value || el.title || el.textContent || '').trim();\n      if (t.toLowerCase() === want) { el.click(); return t; }"
assert old in s
io.open(p, 'w', encoding='utf-8', newline='').write(s.replace(old, new, 1))
print('ok')
