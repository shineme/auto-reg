import json, urllib.request, websocket
NO_PROXY = urllib.request.build_opener(urllib.request.ProxyHandler({}))
port = 1031
raw = NO_PROXY.open("http://127.0.0.1:%s/json/list" % port, timeout=10).read()
pages = [x for x in json.loads(raw) if x.get("type") == "page"]
print("TARGETS:", [(p.get("title","")[:40], p.get("url","")[:80]) for p in pages])
ws = websocket.create_connection(pages[0]["webSocketDebuggerUrl"], timeout=30)
mid = [0]
def send(method, params=None):
    mid[0] += 1
    ws.send(json.dumps({"id": mid[0], "method": method, "params": params or {}}))
    while True:
        m = json.loads(ws.recv())
        if m.get("id") == mid[0]:
            return m
r = send("Runtime.evaluate", {"expression": "document.body.innerText.slice(0,1200)", "returnByValue": True})
print("BODY:", (r.get("result", {}).get("result", {}).get("value") or "")[:1200])
