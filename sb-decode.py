import base64,io,urllib.request,collections
u="https://subs.2519885.dpdns.org/c/?token=ce7b0ad8f20c084448d03891f892d848"
req=urllib.request.Request(u,headers={"User-Agent":"Clash/v1.18.0"})
raw=urllib.request.urlopen(req,timeout=30).read()
txt=raw.decode("utf-8","replace")
dec=base64.b64decode(txt+"="*(-len(txt)%4)).decode("utf-8","replace")
io.open(r"C:\Users\dhl\Desktop\m3\singlebox-sub-decoded.txt","w",encoding="utf-8",newline="\n").write(dec)
lines=[l.strip() for l in dec.split("\n") if l.strip()]
print("total lines",len(lines))
print(collections.Counter(l.split("://")[0] for l in lines))
for l in lines[:5]: print(l[:170])
