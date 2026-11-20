import json
import urllib.error
import urllib.parse
import urllib.request
import http.cookiejar

BASE = "http://web.open-traffic.com"
KEY = "cbe562d176daaffdc4fde0859dffce8599d4b69356b27e39"

jar = http.cookiejar.CookieJar()
opener = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar))
opener.addheaders = [("User-Agent", "Mozilla/5.0"), ("Accept", "*/*")]


def get(url):
    req = urllib.request.Request(url)
    try:
        r = opener.open(req, timeout=30)
        return r.status, r.read().decode(errors="replace")
    except urllib.error.HTTPError as e:
        return e.code, e.read().decode(errors="replace")


def post(url, obj):
    data = json.dumps(obj).encode()
    req = urllib.request.Request(url, data=data,
                                 headers={"Content-Type": "application/json"})
    try:
        r = opener.open(req, timeout=30)
        return r.status, r.read().decode(errors="replace")
    except urllib.error.HTTPError as e:
        return e.code, e.read().decode(errors="replace")


st, body = get(BASE + "/?k=" + urllib.parse.quote(KEY))
print("login:", st, len(body), "cookies:", len(jar))

st, body = get(BASE + "/api/providers")
print("providers:", st, len(body))
try:
    data = json.loads(body)
    provs = data.get("providers", data if isinstance(data, list) else [])
    for p in provs:
        pid = p.get("id", "")
        if "cursor" in pid.lower() or "minimax" in pid.lower():
            print("  provider:", pid, "| name:", p.get("name") or p.get("label"),
                  "| keys:", sorted(list(p.keys()))[:14])
            if pid == "cursor":
                acc = p.get("account") or {}
                print("    cursor.account keys:", sorted(acc.keys()))
                print("    cursor logins:", json.dumps(acc.get("logins"), ensure_ascii=False)[:300])
                print("    cursor user:", acc.get("user"))
except Exception as e:
    print("parse fail:", e, body[:300])

st, body = post(BASE + "/api/plugin-signin",
                {"provider": "cursor", "method": 0, "inputs": {}, "proxy": ""})
print("plugin-signin cursor:", st, body[:600])
