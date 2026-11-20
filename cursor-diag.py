"""诊断 Cursor deep-control 登录页为什么渲染不出来。"""
import json
import re
import shutil
import subprocess
import sys
import tempfile
import time
import urllib.request
import http.cookiejar

PANEL = "http://web.open-traffic.com"
KEY = "cbe562d176daaffdc4fde0859dffce8599d4b69356b27e39"
CHROME = r"C:\Program Files\Google\Chrome\Application\chrome.exe"
NO_PROXY = urllib.request.build_opener(urllib.request.ProxyHandler({}))


def panel_signin():
    cj = http.cookiejar.CookieJar()
    op = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(cj))
    UA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36"
    req = urllib.request.Request(PANEL + "/?k=" + KEY, headers={"User-Agent": UA})
    op.open(req, timeout=30).read()
    body = json.dumps({"provider": "cursor", "method": 0, "inputs": {}, "proxy": ""}).encode()
    req = urllib.request.Request(PANEL + "/api/plugin-signin", data=body,
                                 headers={"Content-Type": "application/json", "User-Agent": UA})
    out = json.loads(op.open(req, timeout=30).read().decode())
    print("[panel] signin:", json.dumps(out, ensure_ascii=False)[:400])
    return out


def free_port():
    import socket
    s = socket.socket()
    s.bind(("127.0.0.1", 0))
    p = s.getsockname()[1]
    s.close()
    return p


class CDP:
    def __init__(self, url):
        import websocket
        self.ws = websocket.create_connection(url, timeout=60)
        self.id = 0

    def send(self, method, params=None):
        self.id += 1
        mid = self.id
        self.ws.send(json.dumps({"id": mid, "method": method, "params": params or {}}))
        while True:
            msg = json.loads(self.ws.recv())
            if msg.get("id") == mid:
                return msg

    def js(self, expr):
        r = self.send("Runtime.evaluate", {"expression": expr, "returnByValue": True})
        res = r.get("result", {}).get("result", {})
        if "value" in res:
            return res["value"]
        return None


def main():
    info = panel_signin()
    url = info["url"]
    port = free_port()
    profile = tempfile.mkdtemp(prefix="cdiag-")
    args = [CHROME, "--remote-debugging-port=%d" % port, "--user-data-dir=" + profile,
            "--no-first-run", "--no-default-browser-check", "--disable-gpu",
            "--window-size=1400,900", url]
    errf = open(tempfile.gettempdir() + "\\cdiag-chrome.err", "wb")
    proc = subprocess.Popen(args, stdout=errf, stderr=errf)
    try:
        ws_url = None
        for _ in range(60):
            time.sleep(0.5)
            try:
                data = json.loads(NO_PROXY.open(
                    "http://127.0.0.1:%d/json/list" % port, timeout=5).read().decode())
                for t in data:
                    if t.get("type") == "page" and t.get("webSocketDebuggerUrl"):
                        ws_url = t["webSocketDebuggerUrl"]
                        break
            except Exception:
                continue
            if ws_url:
                break
        if not ws_url:
            print("no CDP target")
            print("alive:", proc.poll())
            try:
                print(open(tempfile.gettempdir() + "\\cdiag-chrome.err", "rb").read().decode(errors="replace")[-1500:])
            except Exception as e:
                print("no err file", e)
            return
        cdp = CDP(ws_url)
        cdp.send("Runtime.enable")
        cdp.send("Log.enable")
        cdp.send("Network.enable")
        # 页面 hydration 很慢，轮询等按钮出现
        for _ in range(60):
            time.sleep(1)
            btns = cdp.js("Array.from(document.querySelectorAll('button')).map(b=>b.innerText.trim()).filter(Boolean)")
            if btns:
                break
        print("[1] url:", cdp.js("location.href"))
        print("[1] body:", (cdp.js("document.body ? document.body.innerText.slice(0,600) : ''") or "").replace("\n", " | "))
        print("[1] buttons:", cdp.js("Array.from(document.querySelectorAll('button')).map(b=>b.innerText.trim()).filter(Boolean)"))

        # 点 Continue to sign in
        clicked = cdp.js("""
        (function(){
          var els = Array.from(document.querySelectorAll('button,a[role=button],[role=button]'));
          for (var i=0;i<els.length;i++){
            var t = (els[i].innerText||'').trim().toLowerCase();
            if (t.indexOf('continue to sign in')>=0){ els[i].click(); return t; }
          }
          return '';
        })()
        """)
        print("[2] clicked:", clicked)
        for _ in range(40):
            time.sleep(1)
            txt = cdp.js("document.body ? document.body.innerText.slice(0,900) : ''") or ""
            if "email" in txt.lower() or "sign in" in txt.lower() and len(txt) > 200:
                break
        print("[2] url:", cdp.js("location.href"))
        print("[2] body:", (cdp.js("document.body ? document.body.innerText.slice(0,900) : ''") or "").replace("\n", " | "))
        print("[2] inputs:", cdp.js("""
        Array.from(document.querySelectorAll('input')).map(function(i){
          var r=i.getBoundingClientRect();
          return [i.type,i.name,i.id,i.placeholder,i.autocomplete,Math.round(r.width)+'x'+Math.round(r.height)].join('|');
        })
        """))
        print("[2] buttons:", cdp.js("Array.from(document.querySelectorAll('button')).map(b=>b.innerText.trim()).filter(Boolean)"))
        html = cdp.js("document.documentElement.outerHTML.length")
        print("[2] html len:", html)
        errs = cdp.js("""
        (function(){
          var out=[];
          var rs = performance.getEntriesByType('resource');
          return rs.length;
        })()
        """)
        print("[2] resources:", errs)
        cdp.send("Page.captureScreenshot")
    finally:
        proc.terminate()
        try:
            proc.wait(timeout=10)
        except Exception:
            proc.kill()
        shutil.rmtree(profile, ignore_errors=True)


if __name__ == "__main__":
    main()
