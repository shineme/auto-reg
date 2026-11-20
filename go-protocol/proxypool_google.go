package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// googleProbeURL 是 reCAPTCHA Enterprise 实际要访问的域。
// 节点能访问 ipify 只说明链路通，不代表能过 Google 的风控探测。
const googleProbeURL = "https://www.google.com/recaptcha/api2/anchor"

// probeGoogleViaNode 用节点访问 Google reCAPTCHA 域，判断这个出口能不能取到风控票据。
func probeGoogleViaNode(ctx context.Context, node ProxyNodeInfo, mode string) error {
	client, err := nodeHTTPClient(node, mode)
	if err != nil {
		return err
	}
	req, rerr := http.NewRequestWithContext(ctx, http.MethodGet, googleProbeURL, nil)
	if rerr != nil {
		return rerr
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 "+
		"(KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36")
	resp, derr := client.Do(req)
	if derr != nil {
		return derr
	}
	_ = resp.Body.Close()
	// 200/403 都说明 TLS 握手和请求都走通了（403 是 Google 拒绝裸 UA，属正常）。
	if resp.StatusCode == 200 || resp.StatusCode == 403 || resp.StatusCode == 429 {
		return nil
	}
	return fmt.Errorf("google 探测返回异常状态 %d", resp.StatusCode)
}

// nodeHTTPClient 按指定拨号方式建一个走该节点的 HTTP 客户端。
func nodeHTTPClient(node ProxyNodeInfo, mode string) (*http.Client, error) {
	target := node
	switch mode {
	case "http":
		target.Type = "http"
		target.TLS = false
	case "https":
		target.Type = "https"
		target.TLS = true
	case "socks5":
		target.Type = "socks5"
		target.TLS = false
	default:
		return nil, fmt.Errorf("未知拨号方式: %s", mode)
	}
	u := &url.URL{Scheme: target.Type, Host: fmt.Sprintf("%s:%d", target.Server, target.Port)}
	if target.Username != "" {
		u.User = url.UserPassword(target.Username, target.Pass)
	}
	tr := &http.Transport{
		Proxy:                 http.ProxyURL(u),
		TLSHandshakeTimeout:   8 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		DisableKeepAlives:     true,
	}
	return &http.Client{Transport: tr, Timeout: 15 * time.Second}, nil
}
