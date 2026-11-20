import socket,ssl,time
def probe(host,port,tls,mode,target="api.ipify.org:443",extra=""):
    try:
        s=socket.create_connection((host,port),timeout=12)
    except Exception as e:
        return f"tcp fail {e}"
    if tls:
        ctx=ssl.create_default_context(); ctx.check_hostname=False; ctx.verify_mode=ssl.CERT_NONE
        try:
            s=ctx.wrap_socket(s)
        except Exception as e:
            return f"tls fail {e}"
    try:
        if mode=="connect":
            s.sendall(f"CONNECT {target} HTTP/1.1\r\nHost: {target}\r\n{extra}\r\n".encode())
        else:
            s.sendall(f"GET http://{target}/ HTTP/1.1\r\nHost: {target}\r\n{extra}\r\n".encode())
        s.settimeout(12)
        data=s.recv(400)
        return repr(data[:200])
    except Exception as e:
        return f"read fail {e}"
    finally:
        s.close()
print("tls+connect :",probe("138.199.35.196",9002,True,"connect"))
print("tls+get     :",probe("138.199.35.196",9002,True,"get"))
print("plain+connect:",probe("138.199.35.196",9002,False,"connect"))
print("socks5      :",probe("94.194.165.96",62000,False,"connect"))
print("tls443 node :",probe("147.182.216.4",443,True,"connect"))
