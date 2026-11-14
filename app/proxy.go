package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/proxy"
)

type proxyBridge struct {
	mu       sync.Mutex
	server   *http.Server
	listener net.Listener
	upstream string
	url      string
}

func (bridge *proxyBridge) Start(upstream string) (string, error) {
	normalized, err := NormalizeProxyURL(upstream)
	if err != nil {
		return "", err
	}
	bridge.mu.Lock()
	defer bridge.mu.Unlock()
	if bridge.server != nil && bridge.upstream == normalized {
		return bridge.url, nil
	}
	if bridge.server != nil {
		bridge.stopLocked()
	}
	if normalized == "" {
		return "", nil
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	server := &http.Server{
		Handler:           http.HandlerFunc(bridge.connectHandler(normalized)),
		ReadHeaderTimeout: 10 * time.Second,
	}
	bridge.server = server
	bridge.listener = listener
	bridge.upstream = normalized
	bridge.url = fmt.Sprintf("http://127.0.0.1:%d", listener.Addr().(*net.TCPAddr).Port)
	go func() {
		_ = server.Serve(listener)
	}()
	return bridge.url, nil
}

func (bridge *proxyBridge) Stop() {
	bridge.mu.Lock()
	defer bridge.mu.Unlock()
	bridge.stopLocked()
}

func proxyEnv(localProxy string) []string {
	direct := []string{
		"WINHTTP_PROXY=",
		"winhttp_proxy=",
	}
	if localProxy == "" {
		return append(direct,
			"NODE_USE_ENV_PROXY=",
			"HTTP_PROXY=", "HTTPS_PROXY=", "ALL_PROXY=",
			"http_proxy=", "https_proxy=", "all_proxy=",
			"MINIMAX_PROXY_URL=",
			"NO_PROXY=*", "no_proxy=*",
		)
	}
	env := append(direct,
		"NODE_USE_ENV_PROXY=1",
		"NO_PROXY=localhost,127.0.0.1,::1",
		"no_proxy=localhost,127.0.0.1,::1",
	)
	for _, key := range []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "http_proxy", "https_proxy", "all_proxy"} {
		env = append(env, key+"="+localProxy)
	}
	env = append(env, "MINIMAX_PROXY_URL="+localProxy)
	return env
}

func (bridge *proxyBridge) stopLocked() {
	if bridge.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		_ = bridge.server.Shutdown(ctx)
		cancel()
	}
	bridge.server = nil
	bridge.listener = nil
	bridge.upstream = ""
	bridge.url = ""
}

func (bridge *proxyBridge) connectHandler(upstream string) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodConnect {
			http.Error(writer, "only CONNECT is supported", http.StatusMethodNotAllowed)
			return
		}
		target, err := dialUpstream(upstream, request.Host)
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
		go tunnel(client, target)
		go tunnel(target, client)
	}
}

func dialUpstream(upstream, target string) (net.Conn, error) {
	parsed, err := url.Parse(upstream)
	if err != nil {
		return nil, err
	}
	switch parsed.Scheme {
	case "http", "https":
		return dialHTTPProxy(parsed, target)
	case "socks5", "socks5h":
		auth := &proxy.Auth{User: parsed.User.Username()}
		if proxyPassword, ok := parsed.User.Password(); ok {
			auth.Password = proxyPassword
		}
		dialer, err := proxy.SOCKS5("tcp", parsed.Host, auth, &net.Dialer{Timeout: 20 * time.Second})
		if err != nil {
			return nil, err
		}
		return dialer.Dial("tcp", target)
	default:
		return nil, errors.New("unsupported proxy scheme")
	}
}

func dialHTTPProxy(parsed *url.URL, target string) (net.Conn, error) {
	connection, err := net.DialTimeout("tcp", parsed.Host, 20*time.Second)
	if err != nil {
		return nil, err
	}
	request := fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\n", target, target)
	if parsed.User != nil {
		credentials := parsed.User.Username()
		if proxyPassword, ok := parsed.User.Password(); ok {
			credentials += ":" + proxyPassword
		}
		request += "Proxy-Authorization: Basic " + base64.StdEncoding.EncodeToString([]byte(credentials)) + "\r\n"
	}
	request += "\r\n"
	if _, err := io.WriteString(connection, request); err != nil {
		connection.Close()
		return nil, err
	}
	buffer := make([]byte, 4096)
	count, err := connection.Read(buffer)
	if err != nil {
		connection.Close()
		return nil, err
	}
	line := strings.SplitN(string(buffer[:count]), "\r\n", 2)[0]
	if !strings.Contains(line, " 200 ") {
		connection.Close()
		return nil, fmt.Errorf("upstream proxy CONNECT failed: %s", line)
	}
	return connection, nil
}

func tunnel(destination net.Conn, source net.Conn) {
	defer destination.Close()
	defer source.Close()
	_, _ = io.Copy(destination, source)
}
