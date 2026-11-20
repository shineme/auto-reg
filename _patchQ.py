import io
p = r'C:\Users\dhl\Desktop\m3\go-protocol\cursor_flow.go'
s = io.open(p, encoding='utf-8').read()
anchor = "  window.__mmSetAll = function (kind, value) {"
new = """  window.__mmRect = function (text) {
    var want = String(text).trim().toLowerCase();
    var all = Array.prototype.slice.call(document.querySelectorAll('button, a[role=button], [role=button]'));
    for (var i = 0; i < all.length; i++) {
      var el = all[i];
      if (!visible(el)) { continue; }
      if (el.closest && el.closest('#onetrust-consent-sdk, #ot-sdk-container')) { continue; }
      var t = (el.innerText || el.getAttribute('aria-label') || el.value || el.title || '').trim();
      if (t.toLowerCase() === want) {
        var r = el.getBoundingClientRect();
        return { x: r.x + r.width / 2, y: r.y + r.height / 2 };
      }
    }
    return null;
  };
"""
assert anchor in s
io.open(p, 'w', encoding='utf-8', newline='').write(s.replace(anchor, new + anchor, 1))
print('ok')
