import paramiko, json
HOST="64.235.39.49"; USER="root"; PASS="3LskP9pONkBYmLWLsflVMKA1"
c=paramiko.SSHClient(); c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect(HOST,22,username=USER,password=PASS,timeout=20)
def run(cmd,timeout=90):
    i,o,e=c.exec_command(cmd,timeout=timeout)
    out=o.read().decode(errors="replace"); err=e.read().decode(errors="replace")
    if out.strip(): print(out[:1200])
    if err.strip(): print("[err]",err[:400])
    return out
# 1. 读当前 config（VPS 上已有的，避免 BOM 问题）
out=run("head -c 16 /root/.config/minimax-go-protocol/config.json | xxd | head -1")
# 2. 用 python 去掉 BOM 并补全缺失字段，写回干净 UTF-8
fix=r'''python3 - <<EOF
import json
p="/root/.config/minimax-go-protocol/config.json"
raw=open(p,"rb").read()
if raw.startswith(b"\xef\xbb\xbf"): raw=raw[3:]
cfg=json.loads(raw.decode("utf-8"))
cfg["signatureSalt"]="I*7Cf%WZ#S&%1RlZJ&C2"
cfg["yySuffix"]="ooui"
cfg["recaptchaSiteKey"]="{{SECRET_xf7qj2mz}}"
json.dump(cfg,open(p,"w"),indent=2)
print("config fixed:",cfg.get("panelBase"),cfg.get("mailMode"),"salt ok" if cfg["signatureSalt"] else "salt missing")
EOF'''
run(fix)
# 3. 检查 pool 也去 BOM
run("python3 -c \"raw=open('/opt/minimax/hotmail-pool.json','rb').read(); print('pool BOM' if raw.startswith(b'\\xef\\xbb\\xbf') else 'pool clean', len(json.loads(open('/opt/minimax/hotmail-pool.json').read()))['accounts'] if True else 0)\" 2>/dev/null || node -e ''")
run("mkdir -p /opt/minimax/artifacts/go-protocol")
c.close()
