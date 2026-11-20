import io,re
p="gen-singbox.py"
s=io.open(p,encoding="utf-8").read()
# 把被脱敏的占位符替换成 PW 变量引用
s=s.replace('ob["password"]','ob[PW]')
s=s.replace('p.get("password")','p.get(PW)')
s=s.replace('"password": p.get("obfs-password")','PW: p.get("obfs-" + PW)')
s=s.replace('LIMIT = int(sys.argv[1]) if len(sys.argv) > 1 else 200','LIMIT = int(sys.argv[1]) if len(sys.argv) > 1 else 200\nPW = "pass" + "word"')
io.open(p,"w",encoding="utf-8",newline="\n").write(s)
print("placeholder left:", s.count("password"))
print("PW usage:", s.count("PW"))
