package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/net/proxy"
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
	server := &http.Server{Handler: http.HandlerFunc(connectHandler(upstream)), ReadHeaderTimeout: 10 * time.Second}
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
		auth := &proxy.Auth{User: parsed.User.Username()}
		if password, ok := parsed.User.Password(); ok {
			auth.Password = password
		}
		dialer, err := proxy.SOCKS5("tcp", parsed.Host, auth, &net.Dialer{Timeout: 20 * time.Second})
		if err != nil {
			http.Error(writer, err.Error(), http.StatusBadGateway)
			return
		}
		target, err := dialer.Dial("tcp", request.Host)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusBadGateway)
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
