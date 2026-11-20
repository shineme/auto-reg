import socket,ssl
def get(host,port,path):
    ctx=ssl.create_default_context(); ctx.check_hostname=False; ctx.verify_mode=ssl.CERT_NONE
    try:
        s=socket.create_connection((host,port),timeout=12)
        s=ctx.wrap_socket(s)
        s.sendall(f"GET {path} HTTP/1.1\r\nHost: {host}\r\nUser-Agent: Clash/v1.18.0\r\nAccept: */*\r\nConnection: close\r\n\r\n".encode())
        s.settimeout(15)
        buf=b""
        while len(buf)<1500:
            try:
                d=s.recv(1024)
            except Exception as e:
                break
            if not d: break
            buf+=d
        return buf[:900].decode("utf-8","replace")
    except Exception as e:
        return f"ERR {e}"
print(get("138.199.35.215",9002,"/?skip-cert-verify=true&name=test"))
print("=====")
print(get("147.182.216.4",443,"/"))
