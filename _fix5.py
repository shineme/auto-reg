import io
p = r"C:\Users\dhl\Desktop\m3\go-protocol\cursor_flow.go"
s = io.open(p, encoding="utf-8").read()
old = "    return '';\n  };\n  window.__mmSetAll"
new = """    for (var w2 = 0; w2 < words.length; w2++) {
      var sub = words[w2].toLowerCase();
      for (var i2 = 0; i2 < all.length; i2++) {
        var el2 = all[i2];
        if (!visible(el2)) { continue; }
        var t2 = (el2.innerText || el2.value || el2.textContent || '').trim().toLowerCase();
        if (t2 && t2.indexOf(sub) >= 0) { el2.click(); return t2; }
      }
    }
    return '';
  };
  window.__mmSetAll"""
assert old in s
s = s.replace(old, new)
io.open(p, "w", encoding="utf-8").write(s)
print("ok")
