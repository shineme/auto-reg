import yaml, json, io, re, collections
raw=io.open("singlebox-clash.yaml",encoding="utf-8",errors="replace").read()
doc=yaml.safe_load(raw)
proxies=doc.get("proxies",[])
print("total",len(proxies))
c=collections.Counter(p.get("type") for p in proxies)
print(c)
for p in proxies:
    if p.get("type")=="ss":
        print(json.dumps(p,ensure_ascii=False)); break
for p in proxies:
    if p.get("type")=="vmess":
        print(json.dumps(p,ensure_ascii=False)); break
