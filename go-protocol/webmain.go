//go:build web

package main

import (
	"embed"
	"flag"
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
)

//go:embed all:frontend
var webAssets embed.FS

// buildBridge 生成前端桥接脚本：window.go.main.App -> REST，window.runtime.EventsOn -> WebSocket。
func buildBridge(token string) string {
	wsSuffix := ""
	if token != "" {
		wsSuffix = "?token=" + token
	}
	hdr := ""
	if token != "" {
		hdr = `      h["X-MiniMax-Token"] = "` + token + `";`
	}
	return `<script>
(function () {
  var API = "";
  var WS = "ws" + (location.protocol === "https:" ? "s" : "") + "://" + location.host + "/events` + wsSuffix + `";
  var ws = null; var seq = 0; var pending = {}; var events = {};
  function api(name, args) {
    return fetch(API + "/api/call", { method: "POST",
      headers: (function(){ var h={"Content-Type":"application/json"}; ` + hdr + ` return h; })(),
      body: JSON.stringify({ name: name, args: args || [] })
    }).then(function (r) { return r.json(); }).then(function (b) { if (b.error) throw new Error(b.error); return b.data; });
  }
  function connect() {
    ws = new WebSocket(WS);
    ws.onmessage = function (e) { var m = JSON.parse(e.data); (events[m.event] || []).forEach(function (f) { f(m.data); }); };
    ws.onclose = function () { setTimeout(connect, 2000); };
    ws.onerror = function () { try { ws.close(); } catch (e) {} };
  }
  window.go = window.go || {}; window.go.main = window.go.main || {};
  window.go.main.App = {
    GetConfig: function () { return api("GetConfig"); },
    SaveConfig: function (c) { return api("SaveConfig", [c]); },
    GetProgress: function () { return api("GetProgress"); },
    TestPanel: function (c) { return api("TestPanel", [c]); },
    ImportHotmail: function (c) { return api("ImportHotmail", [c]); },
    ImportHotmailFile: function (c, p) { return api("ImportHotmailFile", [c, p]); },
    ImportHotmailText: function (c, t) { return api("ImportHotmailText", [c, t]); },
    GetHotmailStats: function (c) { return api("GetHotmailStats", [c]); },
	ImportProxySubscription: function (c) { return api("ImportProxySubscription", [c]); },
	CheckProxyPool: function (c) { return api("CheckProxyPool", [c]); },
	GetProxyPoolStats: function (c) { return api("GetProxyPoolStats", [c]); },
    PreflightHotmail: function (c) { return api("PreflightHotmail", [c]); },
    RemoveSkippedHotmail: function (c) { return api("RemoveSkippedHotmail", [c]); },
    CheckinAll: function (c) { return api("CheckinAll", [c]); },
    MailScanAll: function (c) { return api("MailScanAll", [c]); },
    MailInboxView: function (c, e) { return api("MailInboxView", [c, e]); },
    MailMessageView: function (c, e, m) { return api("MailMessageView", [c, e, m]); },
    MailListOptions: function (c) { return api("MailListOptions", [c]); },
    StartBatch: function (c) { return api("StartBatch", [c]); },
    StopBatch: function () { return api("StopBatch"); },
    ClearHistory: function (c) { return api("ClearHistory", [c]); },
    StartSupervised: function (c) { return api("StartSupervised", [c]); },
    StopSupervised: function () { return api("StopSupervised"); }
  };
  window.runtime = window.runtime || {};
  window.runtime.EventsOn = function (ev, fn) { (events[ev] = events[ev] || []).push(fn); };
  connect();
})();
</script>`
}

func main() {
	addr := flag.String("addr", "0.0.0.0:8090", "listen address")
	token := flag.String("token", "", "access token (empty = open)")
	flag.Parse()

	app := NewApp()

	raw, _ := webAssets.ReadFile("frontend/index.html")
	page := string(raw)
	if i := strings.LastIndex(page, "</body>"); i > 0 {
		page = page[:i] + "\n" + buildBridge(*token) + page[i:]
	}

	ws := NewWebServer(app, *token)
	handler := withFrontend(ws, page)

	log.Printf("MiniMax web service on http://%s (token: %s)", *addr, tokenState(*token))
	if err := http.ListenAndServe(*addr, handler); err != nil {
		log.Fatal(err)
	}
}

func tokenState(t string) string {
	if t == "" {
		return "off"
	}
	return "on"
}

func withFrontend(ws *WebServer, page string) http.Handler {
	inner := ws.Routes()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/"):
			inner.ServeHTTP(w, r)
		case r.URL.Path == "/events":
			inner.ServeHTTP(w, r)
		case r.URL.Path == "/" || r.URL.Path == "/index.html":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(page))
		default:
			http.NotFound(w, r)
		}
	})
}

var _ = websocket.Upgrader{}
