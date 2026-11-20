import socket,ssl,base64
def tls_get(host,port,path):
    ctx=ssl.create_default_context(); ctx.check_hostname=False; ctx.verify_mode=ssl.CERT_NONE
    try:
        s=socket.create_connection((host,port),timeout=12); s=ctx.wrap_socket(s)
        s.sendall(f"GET {path} HTTP/1.1\r\nHost: {host}:{port}\r\nUser-Agent: Clash/v1.18.0\r\nAccept: */*\r\nConnection: close\r\n\r\n".encode())
        s.settimeout(15); buf=b""
        while True:
            try: d=s.recv(4096)
            except Exception: break
            if not d: break
            buf+=d
        return buf[:1000].decode("utf-8","replace")
    except Exception as e: return f"ERR {e}"
def http_connect_auth(host,port,user,passw,target="api.ipify.org:443"):
    try:
        s=socket.create_connection((host,port),timeout=12)
        cred=base64.b64encode(f"{user}:{passw}".encode()).decode()
        s.sendall(f"CONNECT {target} HTTP/1.1\r\nHost: {target}\r\nProxy-Authorization: Basic {cred}\r\n\r\n".encode())
        s.settimeout(15)
        return repr(s.recv(300))
    except Exception as e: return f"ERR {e}"
print("converter exact:", tls_get("138.199.35.215",9002,"/?skip-cert-verify=true")[:600])
print("---")
print("socks5-labeled node with auth:", http_connect_auth("94.194.165.96",62000,"QyVdo5ks","{{SECRET_6huyk5sw}}"))
