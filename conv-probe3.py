import socket,ssl
host,port="138.199.35.215",9002
def tunnel_test(target="api.ipify.org:443"):
    s=socket.create_connection((host,port),timeout=12)
    ctx=ssl.create_default_context(); ctx.check_hostname=False; ctx.verify_mode=ssl.CERT_NONE
    s=ctx.wrap_socket(s)
    s.sendall(("CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n"%(target,host)).encode())
    s.settimeout(12)
    head=b""
    while b"\r\n\r\n" not in head:
        d=s.recv(1)
        if not d: break
        head+=d
    print("resp head:", head[:120])
    try:
        inner=ssl.SSLContext(ssl.PROTOCOL_TLS_CLIENT)
        inner.check_hostname=False; inner.verify_mode=ssl.CERT_NONE
        t=inner.wrap_socket(s, server_hostname="api.ipify.org")
        t.settimeout(12)
        t.sendall(b"GET / HTTP/1.1\r\nHost: api.ipify.org\r\nConnection: close\r\n\r\n")
        print("inner:", t.recv(400)[:300])
    except Exception as e:
        print("inner ERR:", e)
tunnel_test()
