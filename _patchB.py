import io
p = r'C:\Users\dhl\Desktop\m3\go-protocol\cursor_flow.go'
s = io.open(p, encoding='utf-8').read()
anchor = "  window.__mmSetAll = function (value) {"
if anchor not in s:
    anchor = "  window.__mmSetAll = function (kind, value) {"
new = """  window.__mmFillCode = function (value) {
    var digits = String(value).split('');
    var el = document.querySelector('input[name=code]');
    if (el) { el.focus(); setNativeValue(el, value); }
    var boxes = Array.prototype.slice.call(document.querySelectorAll('input[type=text]'))
      .filter(function (i) { return visible(i) && !i.name; });
    for (var i = 0; i < boxes.length && i < digits.length; i++) {
      setNativeValue(boxes[i], digits[i]);
    }
    if (el) { return true; }
    return boxes.length > 0;
  };
"""
assert anchor in s
io.open(p, 'w', encoding='utf-8', newline='').write(s.replace(anchor, new + anchor, 1))
print('ok')
