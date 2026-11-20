import paramiko, time

HOST="64.235.39.49"; USER="root"; PASS="3LskP9pONkBYmLWLsflVMKA1"
c=paramiko.SSHClient(); c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect(HOST,22,username=USER,password=PASS,timeout=20)
sftp=c.open_sftp()

def run(cmd,timeout=300,show=True):
    i,o,e=c.exec_command(cmd,timeout=timeout)
    out=o.read().decode(errors="replace"); err=e.read().decode(errors="replace")
    if show and out.strip(): print("[out]",out[:3000])
    if show and err.strip(): print("[err]",err[:1500])
    return out,err

# 1. 目录
run("mkdir -p /opt/minimax /root/.config/minimax-go-protocol")
# 2. 上传二进制与池与配置
sftp.put(r"C:\Users\dhl\Desktop\m3\go-protocol\build\bin\minimax-web","/opt/minimax/minimax-web")
sftp.put(r"C:\Users\dhl\Desktop\m3\artifacts\go-protocol\hotmail-pool-1118.json","/opt/minimax/hotmail-pool.json")
sftp.put(r"C:\Users\dhl\Desktop\m3\vps-config.json","/root/.config/minimax-go-protocol/config.json")
print("uploaded")
run("chmod +x /opt/minimax/minimax-web && ls -la /opt/minimax/ && head -c 200 /root/.config/minimax-go-protocol/config.json")
c.close()
