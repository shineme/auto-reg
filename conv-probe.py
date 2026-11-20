import socket,ssl,base64,json
host,port="138.199.35.215",9002
def raw(req, tls=True):
    try:
        s=socket.create_connection((host,port),timeout=12)
        if tls:
            ctx=ssl.create_default_context(); ctx.check_hostname=False; ctx.verify_mode=ssl.CERT_NONE
            s=ctx.wrap_socket(s)
        s.sendall(req.encode())
        s.settimeout(12)
        buf=b""
        while len(buf)<600:
            try: d=s.recv(512)
            except Exception: break
            if not d: break
            buf+=d
        s.close()
        return buf[:500].decode("utf-8","replace")
    except Exception as e:
        return "ERR "+str(e)

target="http://api.ipify.org"
b64=base64.b64encode(target.encode()).decode()
b64url=base64.urlsafe_b64encode(target.encode()).decode().rstrip("=")
print("A) GET /<b64>:", raw(f"GET /{b64} HTTP/1.1\r\nHost: {host}:{port}\r\nConnection: close\r\n\r\n")[:220])
print("B) GET /<b64url>:", raw(f"GET /{b64url} HTTP/1.1\r\nHost: {host}:{port}\r\nConnection: close\r\n\r\n")[:220])
print("C) POST /<b64>:", raw(f"POST /{b64} HTTP/1.1\r\nHost: {host}:{port}\r\nContent-Length: 0\r\nConnection: close\r\n\r\n")[:220])
print("D) GET /:", raw(f"GET / HTTP/1.1\r\nHost: {host}:{port}\r\nConnection: close\r\n\r\n")[:220])
print("E) CONNECT:", raw(f"CONNECT {target} HTTP/1.1\r\nHost: {target}\r\n\r\n")[:220])
