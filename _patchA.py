import io
p = r'C:\Users\dhl\Desktop\m3\go-protocol\cursor_flow.go'
s = io.open(p, encoding='utf-8').read()
old = """    for (var i = 0; i < sels.length; i++) {
      var el = firstVisible(Array.prototype.slice.call(document.querySelectorAll(sels[i])));
      if (el) { return el; }
    }
    return null;
  }"""
new = """    for (var i = 0; i < sels.length; i++) {
      var el = firstVisible(Array.prototype.slice.call(document.querySelectorAll(sels[i])));
      if (el) { return el; }
    }
    if (kind === 'code') {
      var fb = document.querySelector('input[name=code]');
      if (fb) { return fb; }
    }
    return null;
  }"""
assert old in s
io.open(p, 'w', encoding='utf-8', newline='').write(s.replace(old, new, 1))
print('ok')
