import json, urllib.request, websocket
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
expr = """(function(){
  var el = document.elementFromPoint(814, 397);
  var out = {at: el ? el.tagName + '.' + (el.className||'') : null, html: el ? el.outerHTML.slice(0,200) : null};
  var all = document.querySelectorAll('button');
  var cnt = 0, list = [];
  for (var i=0;i<all.length;i++){
    var b = all[i];
    var t = (b.innerText||'').trim();
    if (t.toLowerCase()==='sign in') { cnt++; var r=b.getBoundingClientRect(); list.push({x:r.x,y:r.y,w:r.width,h:r.height,vis:b.offsetParent!==null, pe:getComputedStyle(b).pointerEvents, op:getComputedStyle(b).opacity}); }
  }
  out.signInButtons = list; out.count = cnt;
  return JSON.stringify(out);
})()"""
r = send("Runtime.evaluate", {"expression": expr, "returnByValue": True})
print(r.get("result", {}).get("result", {}).get("value"))
