package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"sync"
	"time"

	"golang.org/x/net/proxy"
)

var (
	stickyProxyMu   sync.Mutex
	stickyProxyUser string
	stickyProxyAt   time.Time
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: proxy-bridge-server <upstream> <port-file>")
		os.Exit(2)
	}
	upstream := os.Args[1]
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	localProxy := fmt.Sprintf("http://127.0.0.1:%d", listener.Addr().(*net.TCPAddr).Port)
	server := &http.Server{Handler: bridgeHandler(upstream), ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = server.Serve(listener) }()
	if err := os.WriteFile(os.Args[2], []byte(localProxy), 0o600); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	<-signals
	_ = server.Close()
}

func rotatingProxyUser(username string) string {
	base := strings.TrimSpace(username)
	if base == "" {
		return base
	}
	lower := strings.ToLower(base)
	if strings.Contains(lower, "-session-") || strings.Contains(lower, "_session-") || strings.Contains(lower, "-sessid-") {
		return base
	}
	stickyProxyMu.Lock()
	defer stickyProxyMu.Unlock()
	if stickyProxyUser == "" {
		suffix := fmt.Sprintf("%d%d", time.Now().UnixNano(), time.Now().Unix()%997)
		stickyProxyUser = base + "-session-" + suffix
		stickyProxyAt = time.Now()
	}
	return stickyProxyUser
}

func rotateStickyProxyUser() {
	stickyProxyMu.Lock()
	defer stickyProxyMu.Unlock()
	stickyProxyUser = ""
	stickyProxyAt = time.Time{}
}

func bridgeHandler(upstream string) http.Handler {
	connect := connectHandler(upstream)
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodConnect {
			connect(writer, request)
			return
		}
		if request.URL.Path == "/__rotate" {
			rotateStickyProxyUser()
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"ok":true,"rotated":true}`))
			return
		}
		http.Error(writer, "only CONNECT or /__rotate is supported", http.StatusMethodNotAllowed)
	})
}

func connectHandler(upstream string) http.HandlerFunc {
	parsed, err := url.Parse(upstream)
	if err != nil {
		return func(writer http.ResponseWriter, request *http.Request) {
			http.Error(writer, err.Error(), http.StatusBadGateway)
		}
	}
	return func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodConnect {
			http.Error(writer, "only CONNECT is supported", http.StatusMethodNotAllowed)
			return
		}
		var target net.Conn
		var dialErr error
		for attempt := 0; attempt < 3; attempt += 1 {
			auth := &proxy.Auth{User: rotatingProxyUser(parsed.User.Username())}
			if password, ok := parsed.User.Password(); ok {
				auth.Password = password
			}
			dialer, err := proxy.SOCKS5("tcp", parsed.Host, auth, &net.Dialer{Timeout: 8 * time.Second})
			if err != nil {
				dialErr = err
				break
			}
			target, dialErr = dialer.Dial("tcp", request.Host)
			if dialErr == nil {
				break
			}
			rotateStickyProxyUser()
			time.Sleep(250 * time.Millisecond)
		}
		if target == nil {
			http.Error(writer, dialErr.Error(), http.StatusBadGateway)
			return
		}
		hijacker, ok := writer.(http.Hijacker)
		if !ok {
			target.Close()
			http.Error(writer, "hijacking unsupported", http.StatusInternalServerError)
			return
		}
		client, buffer, err := hijacker.Hijack()
		if err != nil {
			target.Close()
			return
		}
		if _, err := buffer.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
			client.Close()
			target.Close()
			return
		}
		if err := buffer.Flush(); err != nil {
			client.Close()
			target.Close()
			return
		}
		go copyHalf(client, target)
		go copyHalf(target, client)
	}
}

func copyHalf(destination, source net.Conn) {
	defer destination.Close()
	defer source.Close()
	_, _ = io.Copy(destination, source)
}
