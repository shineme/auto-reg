import paramiko
HOST="64.235.39.49"; USER="root"; PASS="3LskP9pONkBYmLWLsflVMKA1"
c=paramiko.SSHClient(); c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect(HOST,22,username=USER,password=PASS,timeout=20)
def run(cmd,timeout=120):
    i,o,e=c.exec_command(cmd,timeout=timeout)
    out=o.read().decode(errors="replace"); err=e.read().decode(errors="replace")
    if out.strip(): print(out[-1500:])
    if err.strip(): print("[err]",err[-600:])
token="mmweb-"+("".join(__import__("random").choice("abcdefghijklmnopqrstuvwxyz0123456789") for _ in range(16)))
svc=f'''[Unit]
Description=MiniMax Web Registrar
After=network.target

[Service]
Type=simple
Environment=DISPLAY=:99
ExecStartPre=/usr/bin/Xvfb :99 -screen 0 3600x1500x24 &
ExecStart=/opt/minimax/minimax-web -addr 0.0.0.0:8090 -token {token}
Restart=always
RestartSec=5
WorkingDirectory=/opt/minimax

[Install]
WantedBy=multi-user.target
'''
sftp=c.open_sftp()
with sftp.open("/etc/systemd/system/minimax-web.service","w") as f: f.write(svc)
sftp.close()
run("systemctl daemon-reload && systemctl enable --now minimax-web && sleep 3 && systemctl status minimax-web --no-pager | head -8")
run("curl -s http://127.0.0.1:8090/ | head -c 200; echo; echo TOKEN="+token)
c.close()
