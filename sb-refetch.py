import io,urllib.request,base64,urllib.parse,socket,ssl,re
u="https://subs.2519885.dpdns.org/c/?token=ce7b0ad8f20c084448d03891f892d848"
req=urllib.request.Request(u,headers={"User-Agent":"Clash/v1.18.0"})
raw=urllib.request.urlopen(req,timeout=30).read()
io.open("singlebox-clash.yaml","wb").write(raw)
txt=raw.decode("utf-8","replace")
try:
    dec=base64.b64decode(re.sub(r"[^A-Za-z0-9+/=]+","",txt)+"="*(-len(re.sub(r"[^A-Za-z0-9+/=]+","",txt).__len__())%4)).decode("utf-8","replace")
except Exception as e:
    dec=""
io.open("singlebox-sub-decoded.txt","w",encoding="utf-8",newline="\n").write(dec)
lines=[l.strip() for l in dec.split("\n") if l.strip()]
print("decoded lines",len(lines))
print(lines[0][:200])
