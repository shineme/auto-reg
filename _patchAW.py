import io
p = r'C:\Users\dhl\Desktop\m3\go-protocol\cursor_flow.go'
s = io.open(p, encoding='utf-8').read()
old = "        try { el.scrollIntoView({ block: 'center', inline: 'center' }); } catch (e) {}\n        var r = el.getBoundingClientRect();\n        return { x: r.x + r.width / 2, y: r.y + r.height / 2 };"
new = "        try { el.scrollIntoView({ block: 'center', inline: 'center' }); } catch (e) {}\n        var r = el.getBoundingClientRect();\n        var cx = r.x + r.width / 2, cy = r.y + r.height / 2;\n        var top = document.elementFromPoint(cx, cy);\n        if (top && top !== el && !el.contains(top)) { return null; }\n        return { x: cx, y: cy };"
assert old in s
io.open(p, 'w', encoding='utf-8', newline='').write(s.replace(old, new, 1))
print('ok')
