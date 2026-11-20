import json, time, urllib.request, websocket
NO_PROXY = urllib.request.build_opener(urllib.request.ProxyHandler({}))
port = 1031
raw = NO_PROXY.open("http://127.0.0.1:%s/json/list" % port, timeout=10).read()
t = [x for x in json.loads(raw) if x.get("type") == "page"][0]
ws = websocket.create_connection(t["webSocketDebuggerUrl"], timeout=30)
mid = [0]
def send(method, params=None):
    mid[0] += 1
    ws.send(json.dumps({"id": mid[0], "method": method, "params": params or {}}))
    while True:
        m = json.loads(ws.recv())
        if m.get("id") == mid[0]:
            return m
x, y = 716 + 98, 383 + 14
for typ, btn, buttons in (("mouseMoved","none",0),("mousePressed","left",1),("mouseReleased","left",1)):
    send("Input.dispatchMouseEvent", {"type": typ, "x": x, "y": y, "button": btn, "buttons": buttons, "clickCount": 1})
time.sleep(4)
r = send("Runtime.evaluate", {"expression": "location.href + ' || ' + document.title", "returnByValue": True})
print("AFTER CLICK:", r.get("result", {}).get("result", {}).get("value"))
