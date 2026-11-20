import json, sys, urllib.request, subprocess, base64
import websocket
NO_PROXY = urllib.request.build_opener(urllib.request.ProxyHandler({}))
def targets(port):
    raw = NO_PROXY.open("http://127.0.0.1:%s/json/list" % port, timeout=10).read()
    return json.loads(raw)
ports = sys.argv[1:]
for port in ports:
    try:
        for t in targets(port):
            if t.get("type") != "page":
                continue
            ws = websocket.create_connection(t["webSocketDebuggerUrl"], timeout=30)
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
              var all = document.querySelectorAll('button');
              for (var i=0;i<all.length;i++){
                var b=all[i];
                var t=(b.innerText||b.getAttribute('aria-label')||'').trim();
                if (t.toLowerCase()==='sign in'){
                  var r=b.getBoundingClientRect();
                  out.push({html:b.outerHTML.slice(0,300), x:r.x, y:r.y, w:r.width, h:r.height, vis: b.offsetParent!==null});
                }
              }
              return JSON.stringify({url:location.href, title:document.title, btns:out});
            })()"""
            r = send("Runtime.evaluate", {"expression": expr, "returnByValue": True})
            print("PORT", port, json.dumps(r.get("result", {}).get("result", {}).get("value"), ensure_ascii=False)[:900])
            ws.close()
    except Exception as e:
        print("PORT", port, "ERR", e)
