import paramiko
HOST="64.235.39.49"; USER="root"; PASS="3LskP9pONkBYmLWLsflVMKA1"
c=paramiko.SSHClient(); c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect(HOST,22,username=USER,password=PASS,timeout=20)
def run(cmd,timeout=60):
    i,o,e=c.exec_command(cmd,timeout=timeout)
    out=o.read().decode(errors="replace"); err=e.read().decode(errors="replace")
    if out.strip(): print(out[:1500])
    if err.strip(): print("[err]",err[:300])
run("sed -i 's|^Environment=DISPLAY=:99|Environment=DISPLAY=:99\\nEnvironment=HOME=/root|' /etc/systemd/system/minimax-web.service && grep Environment /etc/systemd/system/minimax-web.service")
run("systemctl daemon-reload && systemctl restart minimax-web && sleep 3 && systemctl is-active minimax-web")
run("T=$(awk '/-token/{print $NF}' /etc/systemd/system/minimax-web.service); curl -s -H \"X-MiniMax-Token: $T\" -H 'Content-Type: application/json' -d '{\"name\":\"GetConfig\",\"args\":[]}' http://127.0.0.1:8090/api/call | head -c 400; echo")
run("curl -s -H \"X-MiniMax-Token: $T\" -H 'Content-Type: application/json' -d '{\"name\":\"GetProgress\",\"args\":[]}' http://127.0.0.1:8090/api/call | head -c 300; echo")
c.close()
