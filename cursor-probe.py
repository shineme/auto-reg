import re
import urllib.request
import urllib.error

UA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 " \
     "(KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"


def get(url, headers=None):
    h = {"User-Agent": UA, "Accept": "*/*"}
    if headers:
        h.update(headers)
    req = urllib.request.Request(url, headers=h)
    try:
        r = urllib.request.urlopen(req, timeout=30)
        return r.status, r.read().decode(errors="replace"), dict(r.headers)
    except urllib.error.HTTPError as e:
        return e.code, e.read().decode(errors="replace"), dict(e.headers)
    except Exception as e:
        return -1, str(e), {}


st, body, hdr = get("https://cursor.com/loginDeepControl?challenge=x&uuid=y&mode=login&redirectTarget=cli")
print("loginDeepControl:", st, len(body))
print("server:", hdr.get("Server"), "| cf:", hdr.get("cf-ray", "")[:12])

srcs = re.findall(r'src="([^"]+\.js[^"]*)"', body)
print("scripts:", len(srcs))
paths = set()
for s in srcs[:40]:
    u = s if s.startswith("http") else "https://cursor.com" + s
    st2, js, _ = get(u)
    if st2 != 200:
        continue
    for m in re.findall(r'"(/api/[a-zA-Z0-9_\-/]*)"', js):
        paths.add(m)
    for m in re.findall(r'"(https://[a-z0-9.]*cursor\.sh/[a-zA-Z0-9_\-/]*)"', js):
        paths.add(m)
print("api paths found:")
for p in sorted(paths):
    if any(k in p.lower() for k in ("auth", "login", "sign", "magic", "session", "oauth", "token")):
        print("  ", p)
