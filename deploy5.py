import paramiko, re
HOST="64.235.39.49"; USER="root"; PASS="3LskP9pONkBYmLWLsflVMKA1"
c=paramiko.SSHClient(); c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect(HOST,22,username=USER,password=PASS,timeout=20)
i,o,e=c.exec_command("grep -o 'token [^ ]*' /etc/systemd/system/minimax-web.service | head -1")
m=o.read().decode().split()[-1]
token=m
print("reusing token:", token)
sftp=c.open_sftp()
svc=f'''[Unit]
Description=MiniMax Web Registrar
After=network.target minimax-xvfb.service
Requires=minimax-xvfb.service

[Service]
Type=simple
Environment=DISPLAY=:99
ExecStart=/opt/minimax/minimax-web -addr 0.0.0.0:8090 -token {token}
Restart=always
RestartSec=5
WorkingDirectory=/opt/minimax

[Install]
WantedBy=multi-user.target
'''
xvfb='''[Unit]
Description=Xvfb virtual display for MiniMax Chrome tickets

[Service]
Type=simple
ExecStart=/usr/bin/Xvfb :99 -screen 0 3600x1500x24
Restart=always

[Install]
WantedBy=multi-user.target
'''
with sftp.open("/etc/systemd/system/minimax-xvfb.service","w") as f: f.write(xvfb)
with sftp.open("/etc/systemd/system/minimax-web.service","w") as f: f.write(svc)
sftp.close()
def run(cmd,timeout=60):
    i,o,e=c.exec_command(cmd,timeout=timeout)
    out=o.read().decode(errors="replace"); err=e.read().decode(errors="replace")
    if out.strip(): print(out[:1200])
    if err.strip(): print("[err]",err[:500])
run("systemctl daemon-reload && systemctl enable --now minimax-xvfb && sleep 2 && systemctl restart minimax-web && sleep 3 && systemctl status minimax-web --no-pager | head -6")
run("curl -s -o /dev/null -w 'HTTP_CODE:%{http_code}\\n' http://127.0.0.1:8090/")
c.close()
