import io,re,collections
raw=io.open(r"C:\Users\dhl\Desktop\m3\singlebox-clash.yaml",encoding="utf-8",errors="replace").read()
i=raw.index("proxies:"); j=raw.find("proxy-groups:")
sec=raw[i:j]
blocks=re.split(r"\n    - ",sec)[1:]
rows=[]
for b in blocks:
    d={"name":b.split("\n")[0].strip()}
    for line in b.split("\n")[1:]:
        m=re.match(r"^\s+([A-Za-z\-]+):\s*(.*)$",line)
        if m: d[m.group(1)]=m.group(2).strip().strip('"').strip("'")
    rows.append(d)
httpd=[d for d in rows if d.get("type")=="http" and d.get("server") not in ("127.0.0.1","localhost")]
print("http real",len(httpd))
print("tls:",collections.Counter(d.get("tls") for d in httpd))
print("has user:",collections.Counter(bool(d.get("username")) for d in httpd))
print("ports:",collections.Counter(d.get("port") for d in httpd).most_common(6))
print("sample user entries:")
for d in httpd:
    if d.get("username"): print({k:d.get(k) for k in ("name","server","port","type","tls","username","password","sni")}); break
sk=[d for d in rows if d.get("type")=="socks5" and d.get("server") not in ("127.0.0.1","localhost")]
print("socks5 real",len(sk))
for d in sk[:3]: print({k:d.get(k) for k in ("name","server","port","type","tls","username")})
