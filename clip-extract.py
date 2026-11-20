import re, io, html
s = io.open(r"C:\Users\dhl\Desktop\m3\cliproxy-traffic.html", encoding="utf-8").read()
# strip scripts/styles
s = re.sub(r"(?is)<script.*?</script>", " ", s)
s = re.sub(r"(?is)<style.*?</style>", " ", s)
# keep code blocks as text
s = re.sub(r"(?i)</(p|div|li|tr|h\d|pre|br)>", "\n", s)
s = re.sub(r"(?i)<br\s*/?>", "\n", s)
s = re.sub(r"(?s)<[^>]+>", " ", s)
s = html.unescape(s)
lines = [l.strip() for l in s.split("\n")]
out = []
prev = None
for l in lines:
    if not l:
        continue
    if l == prev:
        continue
    out.append(l)
    prev = l
txt = "\n".join(out)
io.open(r"C:\Users\dhl\Desktop\m3\cliproxy-traffic.txt", "w", encoding="utf-8").write(txt)
print("lines:", len(out))
# print the section that mentions API
for i, l in enumerate(out):
    if any(k in l.lower() for k in ["api", "流量", "traffic", "请求", "header", "token", "url", "示例", "返回"]):
        print(i, l[:200])
