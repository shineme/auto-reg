import paramiko
HOST="64.235.39.49"; USER="root"; PASS="3LskP9pONkBYmLWLsflVMKA1"
c=paramiko.SSHClient(); c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect(HOST,22,username=USER,password=PASS,timeout=20)
def run(cmd,timeout=60):
    i,o,e=c.exec_command(cmd,timeout=timeout)
    out=o.read().decode(errors="replace"); err=e.read().decode(errors="replace")
    if out.strip(): print(out[:1800])
    if err.strip(): print("[err]",err[:300])
run("TOKEN=$(grep -oP '(?<=-token )\\S+' /etc/systemd/system/minimax-web.service); echo TOKEN=$TOKEN; curl -s -H 'X-MiniMax-Token: $TOKEN' -H 'Content-Type: application/json' -d '{\"name\":\"GetConfig\",\"args\":[]}' http://127.0.0.1:8090/api/call | head -c 400; echo")
run("curl -s http://127.0.0.1:8090/ | grep -c 'window.go.main.App = {' ; echo BRIDGE_INJECTED")
run("ufw status 2>/dev/null | head -4; iptables -L INPUT -n 2>/dev/null | head -8")
c.close()
