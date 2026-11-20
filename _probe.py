import json, sys, urllib.request, websocket
NO_PROXY = urllib.request.build_opener(urllib.request.ProxyHandler({}))
port = sys.argv[1]
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
expr = "(function(){var bs=document.querySelectorAll('button');var out=null;for(var i=0;i<bs.length;i++){var t=(bs[i].innerText||'').trim();if(t.toLowerCase()==='sign in'){var r=bs[i].getBoundingClientRect();out={x:r.x,y:r.y,w:r.width,h:r.height,pe:getComputedStyle(bs[i]).pointerEvents};}}var el=document.elementFromPoint(814,397);return JSON.stringify({btn:out,at:el?el.tagName+'|'+(el.innerText||'').slice(0,20):null,scrollY:window.scrollY,ih:window.innerHeight});})()"
r = send("Runtime.evaluate", {"expression": expr, "returnByValue": True})
print(r.get("result", {}).get("result", {}).get("value"))
