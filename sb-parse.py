import io,collections,re
raw=io.open(r"C:\Users\dhl\Desktop\m3\singlebox-clash.yaml",encoding="utf-8",errors="replace").read()
# crude parse of proxies blocks
blocks=re.findall(r"^    - name: (.*)$",raw,re.M)
print("proxy entries:",len(blocks))
srv=re.findall(r"^      server: (.*)$",raw,re.M)
port=re.findall(r"^      port: (.*)$",raw,re.M)
typ=re.findall(r"^      type: (.*)$",raw,re.M)
print("servers distinct:",collections.Counter(srv).most_common(10))
print("types:",collections.Counter(typ).most_common())
print("ports:",collections.Counter(port).most_common(10))
print("---- group/rules")
print(raw[-2500:])
