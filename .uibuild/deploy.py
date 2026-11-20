import paramiko, time, json, urllib.request

HOST = "64.235.39.49"
TOKEN = "3LskP9pONkBYmLWLsflVMKA1"

c = paramiko.SSHClient(); c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect(HOST, 22, username="root", password=TOKEN, timeout=20, banner_timeout=300)

def run(cmd, timeout=300):
    i, o, e = c.exec_command(cmd, timeout=timeout)
    return o.read().decode(errors="replace") + e.read().decode(errors="replace")

tok = run("awk '/-token/{print $NF}' /etc/systemd/system/minimax-web.service").strip().split("\n")[-1]

def api(name, args=None, timeout=120):
    body = json.dumps({"name": name, "args": args or []}).encode()
    req = urllib.request.Request("http://%s:8090/api/call" % HOST, data=body,
                                 headers={"Content-Type": "application/json", "X-MiniMax-Token": tok})
    raw = urllib.request.urlopen(req, timeout=timeout).read().decode()
    return json.loads(raw)

before = api("GetProgress").get("data", {})
print("before:", {k: before.get(k) for k in ("target", "succeeded", "failedAttempts", "running")})

cfg = api("GetConfig").get("data", {})
print("config keys:", sorted(cfg.keys())[:8], "... panelBase=", cfg.get("panelBase"), "count=", cfg.get("count"), "workers=", cfg.get("workers"))

run("systemctl stop minimax-web; sleep 2")
sftp = c.open_sftp()
sftp.put(r"C:\Users\dhl\Desktop\m3\go-protocol\build\bin\minimax-web", "/opt/minimax/minimax-web.new")
sftp.close()
print(run("mv -f /opt/minimax/minimax-web.new /opt/minimax/minimax-web && chmod 755 /opt/minimax/minimax-web && systemctl start minimax-web && sleep 6 && systemctl is-active minimax-web").strip())

page = run("curl -s -m 20 -H 'X-MiniMax-Token: %s' http://127.0.0.1:8090/ | grep -c '协议注册工作台'" % tok).strip()
print("new frontend served:", page)

res = api("StartSupervised", [cfg])
print("StartSupervised:", str(res)[:200])
time.sleep(20)
after = api("GetProgress").get("data", {})
print("after:", {k: after.get(k) for k in ("target", "succeeded", "failedAttempts", "running", "currentStage")})
print("magpie alive:", run("ps aux | grep -c '[m]agpie'").strip())
c.close()
