import io,urllib.request,base64,re
u="https://subs.2519885.dpdns.org/c/?token=ce7b0ad8f20c084448d03891f892d848"
req=urllib.request.Request(u,headers={"User-Agent":"v2rayN/6.0"})
raw=urllib.request.urlopen(req,timeout=30).read()
txt=raw.decode("utf-8","replace")
clean=re.sub(r"[^A-Za-z0-9+/=]+","",txt)
dec=base64.b64decode(clean+"="*(-len(clean)%4)).decode("utf-8","replace")
io.open("singlebox-sub-decoded.txt","w",encoding="utf-8",newline="\n").write(dec)
lines=[l.strip() for l in dec.split("\n") if l.strip()]
print("decoded lines",len(lines))
for l in lines[:3]: print(l[:200])
