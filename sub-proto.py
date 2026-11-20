import io,collections,urllib.parse
lines=[l.strip() for l in io.open("singlebox-sub-decoded.txt",encoding="utf-8",errors="replace").read().split("\n") if l.strip()]
print("total",len(lines))
c=collections.Counter(l.split("://")[0].lower() for l in lines)
print(c)
# 看 http 链接的 query 参数
qs=collections.Counter()
hosts=collections.Counter()
for l in lines:
    if l.lower().startswith("http"):
        u=urllib.parse.urlsplit(l)
        qs[u.query]+=1
        hosts[u.netloc]+=1
print("query variants:",qs.most_common(5))
print("distinct hosts:",len(hosts), hosts.most_common(5))
