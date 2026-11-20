package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
	"path/filepath"
	"os"
	"io"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
)

// WebServer 把 App 引擎暴露为 REST API + WebSocket 事件流，前端复用桌面版同一套 UI。
type WebServer struct {
	app      *App
	upgrader websocket.Upgrader
	token    string

	mu  sync.RWMutex
	hub *wsHub
}

type wsHub struct {
	mu   sync.Mutex
	cons map[chan []byte]struct{}
}

func newWshub() *wsHub {
	return &wsHub{cons: map[chan []byte]struct{}{}}
}

func (h *wsHub) join() chan []byte {
	ch := make(chan []byte, 64)
	h.mu.Lock()
	h.cons[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

func (h *wsHub) leave(ch chan []byte) {
	h.mu.Lock()
	delete(h.cons, ch)
	h.mu.Unlock()
}

func (h *wsHub) broadcast(payload []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.cons {
		select {
		case ch <- payload:
		default:
		}
	}
}

func NewWebServer(app *App, token string) *WebServer {
	s := &WebServer{app: app, token: token, hub: newWshub()}
	s.upgrader = websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	SetEventSink(func(event string, data any) {
		payload, err := json.Marshal(map[string]any{"event": event, "data": data})
		if err == nil {
			s.hub.broadcast(payload)
		}
	})
	return s
}

// authorized 校验访问令牌（未配置令牌时全开）。
func (s *WebServer) authorized(r *http.Request) bool {
	if s.token == "" {
		return true
	}
	if r.Header.Get("X-MiniMax-Token") == s.token {
		return true
	}
	return r.URL.Query().Get("token") == s.token
}

type webAPIReq struct {
	Name string            `json:"name"`
	Args []json.RawMessage `json:"args"`
}

func (s *WebServer) dispatch(name string, args []json.RawMessage) (any, error) {
	// 以已保存配置为基底，请求里的 cfg 仅覆盖其提供的字段，避免部分参数冲掉已存配置
	cfg, _ := LoadConfig()
	email, msgID := "", ""
	if len(args) > 0 {
		_ = json.Unmarshal(args[0], &cfg)
	}
	if len(args) > 1 {
		_ = json.Unmarshal(args[1], &email)
	}
	if len(args) > 2 {
		_ = json.Unmarshal(args[2], &msgID)
	}
	cfg.applyDefaults()
	switch name {
	case "GetConfig":
		return s.app.GetConfig()
	case "SaveConfig":
		return s.app.SaveConfig(cfg)
	case "GetProgress":
		return s.app.GetProgress(), nil
	case "GetProgressLite":
		return s.app.GetProgressLite(), nil
	case "TestPanel":
		return s.app.TestPanel(cfg)
	case "ImportHotmail":
		return s.app.ImportHotmail(cfg)
	case "ImportHotmailFile":
		path := ""
		if len(args) > 1 {
			_ = json.Unmarshal(args[1], &path)
		}
		return s.app.ImportHotmailFile(cfg, path)
	case "GetHotmailStats":
		return s.app.GetHotmailStats(cfg)
	case "ImportProxySubscription":
		return s.app.ImportProxySubscription(cfg)
	case "CheckProxyPool":
		return s.app.CheckProxyPool(cfg)
	case "GetProxyPoolStats":
		return s.app.GetProxyPoolStats(cfg)
	case "PreflightHotmail":
		return s.app.PreflightHotmail(cfg)
	case "RemoveSkippedHotmail":
		return s.app.RemoveSkippedHotmail(cfg)
	case "CheckinAll":
		return s.app.CheckinAll(cfg)
	case "MailScanAll":
		return s.app.MailScanAll(cfg)
	case "MailInboxView":
		return s.app.MailInboxView(cfg, email)
	case "MailMessageView":
		return s.app.MailMessageView(cfg, email, msgID)
	case "MailListOptions":
		return s.app.MailListOptions(cfg)
	case "SearchMailboxes":
		query := ""
		if len(args) > 1 {
			_ = json.Unmarshal(args[1], &query)
		}
		limit := 50
		if len(args) > 2 {
			_ = json.Unmarshal(args[2], &limit)
		}
		return s.app.SearchMailboxes(cfg, query, limit)
	case "GetResultsPage":
		offset, limit := 0, 50
		query, sortBy := "", ""
		if len(args) > 1 {
			_ = json.Unmarshal(args[1], &offset)
		}
		if len(args) > 2 {
			_ = json.Unmarshal(args[2], &limit)
		}
		if len(args) > 3 {
			_ = json.Unmarshal(args[3], &query)
		}
		if len(args) > 4 {
			_ = json.Unmarshal(args[4], &sortBy)
		}
		return s.app.GetResultsPage(cfg, offset, limit, query, sortBy)
	case "ClearHistory":
		return nil, s.app.ClearHistory(cfg)
	case "StartSupervised":
		if err := s.app.StartSupervised(cfg); err != nil {
			return nil, err
		}
		return nil, nil
	case "StopSupervised":
		s.app.StopSupervised()
		return nil, nil
	case "StartBatch":
		if err := s.app.StartBatch(cfg); err != nil {
			return nil, err
		}
		return nil, nil
	case "StopBatch":
		s.app.StopBatch()
		return nil, nil
	case "StartCursorBinding":
		count := 0
		if len(args) > 1 {
			_ = json.Unmarshal(args[1], &count)
		}
		if err := s.app.StartCursorBinding(cfg, count); err != nil {
			return nil, err
		}
		return nil, nil
	case "StopCursorBinding":
		s.app.StopCursorBinding()
		return nil, nil
	case "GetCursorProgress":
		return s.app.GetCursorProgress(), nil
	case "GetPlatformStatus":
		return s.app.GetPlatformStatus(cfg), nil
	}
	return nil, fmt.Errorf("unknown api: %s", name)
}

func (s *WebServer) handleAPI(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req webAPIReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	data, err := s.dispatch(req.Name, req.Args)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err != nil {
		_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
}

func (s *WebServer) handleEvents(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	ch := s.hub.join()
	defer func() {
		s.hub.leave(ch)
		_ = conn.Close()
	}()
	writeDone := make(chan struct{})
	go func() {
		defer close(writeDone)
		for payload := range ch {
			if werr := conn.WriteMessage(websocket.TextMessage, payload); werr != nil {
				return
			}
		}
	}()
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()
	<-readDone
	<-writeDone
}

// handleUpload 接收浏览器上传的号池文本文件，落到服务器本地后返回路径与行数。
// 大文本推荐走这条通道：不受浏览器粘贴/传输截断影响。
func (s *WebServer) handleUpload(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	cfg, _ := LoadConfig()
	cfg.applyDefaults()
	dir := filepath.Join(cfg.OutputDir, "imports")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// 限制单次上传大小（100MB 足够数十万条账号）
	r.Body = http.MaxBytesReader(w, r.Body, 100<<20)
	if err := r.ParseMultipartForm(16 << 20); err != nil {
		http.Error(w, "upload failed: "+err.Error(), http.StatusBadRequest)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "no file: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()
	name := filepath.Base(header.Filename)
	if name == "" || name == "." || name == string(filepath.Separator) {
		name = "hotmail-import.txt"
	}
	target := filepath.Join(dir, fmt.Sprintf("%d-%s", time.Now().Unix(), name))
	dst, err := os.Create(target)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer dst.Close()
	written, err := io.Copy(dst, file)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	lines := 0
	if data, rerr := os.ReadFile(target); rerr == nil {
		lines = strings.Count(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") + 1
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"path": target, "bytes": written, "lines": lines})
}

// Routes 返回带 /api 与 /events 的 handler。
func (s *WebServer) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/upload", s.handleUpload)
	mux.HandleFunc("/api/", s.handleAPI)
	mux.HandleFunc("/events", s.handleEvents)
	return mux
}

var _ = strings.TrimSpace
