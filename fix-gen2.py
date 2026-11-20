import io
p="gen-singbox.py"
s=io.open(p,encoding="utf-8").read()
s=s.replace('''    t = p.get("type")
    ob = {"type": t, "tag": tag, "server": p["server"], "server_port": int(p["port"])}''','''    t = p.get("type")
    sb_type = {"ss": "shadowsocks"}.get(t, t)
    ob = {"type": sb_type, "tag": tag, "server": p["server"], "server_port": int(p["port"])}''')
io.open(p,"w",encoding="utf-8",newline="\n").write(s)
print("ok")
