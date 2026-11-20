import json, time, urllib.request, websocket
NO_PROXY = urllib.request.build_opener(urllib.request.ProxyHandler({}))
port = 1031
raw = NO_PROXY.open("http://127.0.0.1:%s/json/list" % port, timeout=10).read()
pages = [x for x in json.loads(raw) if x.get("type") == "page"]
ws = websocket.create_connection(pages[0]["webSocketDebuggerUrl"], timeout=30)
mid = [0]
def send(method, params=None):
    mid[0] += 1
    ws.send(json.dumps({"id": mid[0], "method": method, "params": params or {}}))
    while True:
        m = json.loads(ws.recv())
        if m.get("id") == mid[0]:
            return m
send("Network.enable"); send("Runtime.enable"); send("Log.enable")
events = []
ws.settimeout(1.0)
x, y = 716 + 98, 383 + 14
for typ, btn, buttons in (("mouseMoved","none",0),("mousePressed","left",1),("mouseReleased","left",1)):
    send("Input.dispatchMouseEvent", {"type": typ, "x": x, "y": y, "button": btn, "buttons": buttons, "clickCount": 1})
end = time.time() + 8
while time.time() < end:
    try:
        m = json.loads(ws.recv())
    except Exception:
        continue
    meth = m.get("method","")
    if meth in ("Network.requestWillBeSent","Network.responseReceived","Runtime.consoleAPICalled","Runtime.exceptionThrown","Log.entryAdded","Network.loadingFailed"):
        p = m.get("params",{})
        if meth == "Network.requestWillBeSent":
            events.append(("REQ", p.get("request",{}).get("method"), p.get("request",{}).get("url","")[:110]))
        elif meth == "Network.responseReceived":
            events.append(("RES", p.get("response",{}).get("status"), p.get("response",{}).get("url","")[:110]))
        elif meth == "Runtime.consoleAPICalled":
            events.append(("CON", str(p.get("args"))[:200]))
        elif meth == "Runtime.exceptionThrown":
            events.append(("EXC", str(p.get("exceptionDetails"))[:250]))
        elif meth == "Log.entryAdded":
            events.append(("LOG", str(p.get("entry"))[:200]))
        else:
            events.append(("FAIL", p.get("errorText")))
for e in events[-40:]:
    print(e)
