import io,re
raw=io.open("singlebox-clash.yaml",encoding="utf-8",errors="replace").read()
i=raw.index("proxies:"); j=raw.find("proxy-groups:")
sec=raw[i:j]
blocks=re.split(r"\n    - ",sec)[1:]
seen=set()
for b in blocks:
    m=re.search(r"type:\s*(\w+)",b)
    if not m: continue
    t=m.group(1)
    if t in ("http","socks5"): continue
    if t in seen: continue
    seen.add(t)
    print("=====",t)
    print(b[:900])
