import io
p="gen-singbox.py"
s=io.open(p,encoding="utf-8").read()
s=s.replace('''    fp = p.get("client-fingerprint")
    if fp:
        t["utls"] = {"enabled": True, "fingerprint": fp}''','''    fp = p.get("client-fingerprint")
    if fp and fp not in ("unsafe", ""):
        t["utls"] = {"enabled": True, "fingerprint": fp}''')
io.open(p,"w",encoding="utf-8",newline="\n").write(s)
