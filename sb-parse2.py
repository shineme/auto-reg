import io,re,collections
raw=io.open(r"C:\Users\dhl\Desktop\m3\singlebox-clash.yaml",encoding="utf-8",errors="replace").read()
# split proxies section
i=raw.index("proxies:")
j=raw.find("proxy-groups:")
sec=raw[i:j]
blocks=re.split(r"\n    - ", sec)
print("blocks",len(blocks))
out=[]
for b in blocks[1:]:
    d={}
    for line in b.split("\n"):
        m=re.match(r"^      ([A-Za-z\-]+): (.*)$",line.strip() and line or line)
        if m: d[m.group(1)]=m.group(2).strip()
        m2=re.match(r"^      - (.*)$",line)
    name=b.split("\n")[0].strip()
    d["name"]=name
    out.append(d)
print(collections.Counter(d.get("type") for d in out))
httpd=[d for d in out if d.get("type") in ("http","socks5")]
print("http/socks5 total",len(httpd))
real=[d for d in httpd if d.get("server") not in ("127.0.0.1","localhost")]
print("real server",len(real))
for d in real[:8]:
    print({k:v for k,v in d.items() if k in ("name","server","port","type","username","password","tls","sni","skip-cert-verify","udp")})
print("--- those with user/pass:",sum(1 for d in real if d.get("username")))
print("--- unique server:port:",len({(d.get("server"),d.get("port")) for d in real}))
