import os
import re
import urllib.request
import urllib.error

UA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 " \
     "(KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
OUT = r"C:\Users\dhl\Desktop\m3\.uibuild\cursorjs"
os.makedirs(OUT, exist_ok=True)


def get(url):
    req = urllib.request.Request(url, headers={"User-Agent": UA, "Accept": "*/*"})
    try:
        r = urllib.request.urlopen(req, timeout=30)
        return r.read().decode(errors="replace")
    except Exception:
        return ""


page = get("https://cursor.com/loginDeepControl?challenge=x&uuid=y&mode=login&redirectTarget=cli")
srcs = re.findall(r'src="([^"]+\.js[^"]*)"', page)
print("scripts:", len(srcs))
hits = 0
for i, s in enumerate(srcs):
    u = s if s.startswith("http") else "https://cursor.com" + s
    js = get(u)
    if not js:
        continue
    name = u.rsplit("/", 1)[-1].split("?")[0]
    path = os.path.join(OUT, "%02d_%s" % (i, name))
    with open(path, "w", encoding="utf-8") as f:
        f.write(js)
    if "loginDeep" in js:
        hits += 1
        print("HIT", path, len(js))
print("files:", len(os.listdir(OUT)), "hits:", hits)
