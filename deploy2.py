import paramiko, time
HOST="64.235.39.49"; USER="root"; PASS="3LskP9pONkBYmLWLsflVMKA1"
c=paramiko.SSHClient(); c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect(HOST,22,username=USER,password=PASS,timeout=20)
def run(cmd,timeout=900):
    i,o,e=c.exec_command(cmd,timeout=timeout)
    out=o.read().decode(errors="replace"); err=e.read().decode(errors="replace")
    print("$",cmd[:120])
    if out.strip(): print(out[-2500:])
    if err.strip(): print("[err]",err[-800:])
run("apt-get update -qq 2>&1 | tail -2")
run("wget -q -O /tmp/chrome.deb https://dl.google.com/linux/direct/google-chrome-stable_current_amd64.deb && echo GOT_DEB")
run("apt-get install -y -qq /tmp/chrome.deb 2>&1 | tail -5")
run("apt-get install -y -qq xvfb 2>&1 | tail -3")
run("google-chrome --version 2>&1; which Xvfb")
c.close()
