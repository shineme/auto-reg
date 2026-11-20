import io,urllib.request
u="https://subs.2519885.dpdns.org/c/?token=ce7b0ad8f20c084448d03891f892d848"
for ua in ["Clash/v1.18.0"]:
    req=urllib.request.Request(u,headers={"User-Agent":ua})
    raw=urllib.request.urlopen(req,timeout=30).read()
    io.open(r"C:\Users\dhl\Desktop\m3\singlebox-clash.yaml","wb").write(raw)
    print(ua,"len",len(raw))
    print(raw[:600].decode("utf-8","replace"))
