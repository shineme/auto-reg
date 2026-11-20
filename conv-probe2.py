import socket,ssl
host,port="138.199.35.215",9002
def probe(hostheader, target="api.ipify.org:443"):
    try:
        s=socket.create_connection((host,port),timeout=12)
        ctx=ssl.create_default_context(); ctx.check_hostname=False; ctx.verify_mode=ssl.CERT_NONE
        s=ctx.wrap_socket(s)
        s.sendall(f"CONNECT {target} HTTP/1.1\r\nHost: {hostheader}\r\n\r\n".encode())
        s.settimeout(12)
        buf=b""
        while len(buf)<300:
            try: d=s.recv(300)
            except Exception: break
            if not d: break
            buf+=d
        return buf[:250].decode("utf-8","replace")
    except Exception as e:
        return "ERR "+str(e)
print("1 host=self:ip   :", probe(f"{host}:{port}"))
print("2 host=self      :", probe(host))
print("3 host=ip:port   :", probe(f"{host}:{port}"))
