import io,re
raw=io.open(r"C:\Users\dhl\Desktop\m3\singlebox-clash.yaml",encoding="utf-8",errors="replace").read()
i=raw.index("proxies:"); j=raw.find("proxy-groups:")
sec=raw[i:j]
blocks=re.split(r"\n    - ",sec)[1:]
n=0
for b in blocks:
    if "138.199.35.196" in b or "94.194.165.96" in b:
        print("----")
        print(b[:500])
        n+=1
        if n>3: break
