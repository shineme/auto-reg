package main

import (
	"net/url"
	"testing"
)

func TestNormalizeProxyURLForms(t *testing.T) {
	testCases := []struct {
		input    string
		scheme   string
		username string
		host     string
		port     string
	}{
		{"socks5://example-user:pass@proxy.example.com:10000", "socks5", "example-user", "proxy.example.com", "10000"},
		{"socks5://example-user:pass:proxy.example.com:10000", "socks5", "example-user", "proxy.example.com", "10000"},
		{`http\://example-user:pass:proxy.example.com:10000`, "http", "example-user", "proxy.example.com", "10000"},
	}
	for _, testCase := range testCases {
		normalized, err := NormalizeProxyURL(testCase.input)
		if err != nil {
			t.Fatalf("NormalizeProxyURL(%q): %v", testCase.input, err)
		}
		parsed, err := url.Parse(normalized)
		if err != nil {
			t.Fatalf("url.Parse(%q): %v", normalized, err)
		}
		if parsed.Scheme != testCase.scheme || parsed.User.Username() != testCase.username || parsed.Hostname() != testCase.host || parsed.Port() != testCase.port {
			t.Fatalf("NormalizeProxyURL(%q) = %q", testCase.input, normalized)
		}
	}
}
