import socket,ssl,urllib.parse
lines=[l.strip() for l in open("singlebox-sub-decoded.txt",encoding="utf-8",errors="replace") if l.strip()]
line=[l for l in lines if "138.199.35.215" in l]
print("count",len(line))
u=line[0]
p=urllib.parse.urlsplit(u)
print("scheme",p.scheme,"host",p.hostname,"port",p.port)
print("query len",len(p.query))
def get(host,port,path,query):
    ctx=ssl.create_default_context(); ctx.check_hostname=False; ctx.verify_mode=ssl.CERT_NONE
    s=socket.create_connection((host,port),timeout=12); s=ctx.wrap_socket(s)
    req=f"GET {path}?{query} HTTP/1.1\r\nHost: {host}:{port}\r\nUser-Agent: Clash/v1.18.0\r\nConnection: close\r\n\r\n"
    s.sendall(req.encode()); s.settimeout(15)
    buf=b""
    while True:
        try: d=s.recv(4096)
        except Exception: break
        if not d: break
        buf+=d
    return buf[:1200].decode("utf-8","replace")
print(get(p.hostname,p.port or 9002,p.path,p.query)[:1200])
