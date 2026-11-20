import paramiko
HOST="64.235.39.49"; USER="root"; PASS="3LskP9pONkBYmLWLsflVMKA1"
c=paramiko.SSHClient(); c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect(HOST,22,username=USER,password=PASS,timeout=20)
def run(cmd,timeout=90):
    i,o,e=c.exec_command(cmd,timeout=timeout)
    out=o.read().decode(errors="replace"); err=e.read().decode(errors="replace")
    if out.strip(): print(out[:1500])
    if err.strip(): print("[err]",err[:300])
    return out
sftp=c.open_sftp()
sftp.put(r"C:\Users\dhl\Desktop\m3\vps-config.json","/root/.config/minimax-go-protocol/config.json")
out=run("python3 -c \"import json; c=json.load(open('/root/.config/minimax-go-protocol/config.json')); print('panelKey ok:',bool(c.get('panelKey')),'salt:',c.get('signatureSalt')[:6],'sitekey:',bool(c.get('recaptchaSiteKey')),'chrome:',c.get('chromePath'),'count:',c.get('count'),'mode:',c.get('mailMode'))\"")
run("systemctl restart minimax-web && sleep 3 && systemctl is-active minimax-web")
c.close()
