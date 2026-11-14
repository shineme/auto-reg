package main

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"
)

func TestProxyBridgeCONNECTThroughSOCKS5(t *testing.T) {
	raw := testProxyURL()
	if raw == "" {
		t.Skip("MINIMAX_TEST_PROXY_URL is not configured")
	}
	normalized, err := NormalizeProxyURL(raw)
	if err != nil {
		t.Fatalf("NormalizeProxyURL: %v", err)
	}
	bridge := &proxyBridge{}
	defer bridge.Stop()
	localProxy, err := bridge.Start(normalized)
	if err != nil {
		t.Fatalf("bridge.Start: %v", err)
	}
	proxyAddress, err := url.Parse(localProxy)
	if err != nil {
		t.Fatalf("url.Parse(%q): %v", localProxy, err)
	}
	client := &http.Client{
		Transport: &http.Transport{Proxy: http.ProxyURL(proxyAddress)},
		Timeout:   20 * time.Second,
	}
	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://www.google.com/generate_204", nil)
	if err != nil {
		t.Fatalf("http.NewRequest: %v", err)
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("proxy CONNECT request: %v", err)
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, response.Body)
	if response.StatusCode != http.StatusNoContent && response.StatusCode != http.StatusOK {
		t.Fatalf("proxy CONNECT status = %d", response.StatusCode)
	}
}

func testProxyURL() string {
	return os.Getenv("MINIMAX_TEST_PROXY_URL")
}
