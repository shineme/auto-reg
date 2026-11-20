import json, urllib.request, websocket
NO_PROXY = urllib.request.build_opener(urllib.request.ProxyHandler({}))
port = 43123
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
expr = """(function(){
  var out = [];
  var els = document.querySelectorAll('a, button, [role=button], input[type=submit]');
  for (var i=0;i<els.length;i++){
    var e = els[i];
    var r = e.getBoundingClientRect();
    if (r.width<=0||r.height<=0) continue;
    out.push({tag:e.tagName, txt:(e.innerText||e.getAttribute('aria-label')||'').trim().slice(0,40), href:(e.getAttribute('href')||'').slice(0,80), pe:getComputedStyle(e).pointerEvents, x:Math.round(r.x), y:Math.round(r.y)});
  }
  return JSON.stringify({url:location.href, body:(document.body.innerText||'').slice(0,900), els:out});
})()"""
r = send("Runtime.evaluate", {"expression": expr, "returnByValue": True})
d = json.loads(r.get("result", {}).get("result", {}).get("value"))
print("URL:", d["url"])
print("BODY:", d["body"])
for e in d["els"]:
    print(" ", e)

