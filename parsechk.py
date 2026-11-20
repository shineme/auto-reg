import io,re
f=r"C:\Users\dhl\Downloads\{万起350}全新微软邮箱Hotmail令牌号支持OAuth2令牌和Graph令牌双协议登录已开通I_20260915_854条_20261120_001547.txt"
raw=io.open(f,encoding="utf-8",errors="replace").read()
raw=raw.replace("\r\n","\n")
uuid=re.compile(r"(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$")
ok=0;skip=0
for line in raw.split("\n"):
    line=line.strip()
    if not line or line.startswith("#"): continue
    parts=[p.strip() for p in line.split("----")]
    if len(parts)<4: skip+=1; continue
    email,pwd,cid=parts[0],parts[1],parts[2]
    rt="----".join(parts[3:])
    if "@" not in email or not pwd or not cid or not rt: skip+=1; continue
    if not uuid.match(cid) and len(cid)<8: skip+=1; continue
    ok+=1
print("parse ok:",ok,"skip:",skip)
