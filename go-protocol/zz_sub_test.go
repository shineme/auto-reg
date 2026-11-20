package main

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSplitSubscriptionURLs(t *testing.T) {
	raw := "https://a/sub?token=1\nhttps://b/sub?token=2, https://a/sub?token=1\n;https://c/sub?token=3 "
	got := splitSubscriptionURLs(raw)
	want := []string{
		"https://a/sub?token=1",
		"https://b/sub?token=2",
		"https://c/sub?token=3",
	}
	if len(got) != len(want) {
		t.Fatalf("数量不对: got %d (%v), want %d", len(got), got, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("第 %d 项不对: got %q want %q", i, got[i], want[i])
		}
	}
}

func TestProxyNodeKeyDedup(t *testing.T) {
	a := ProxyNodeInfo{Type: "socks5", Server: "1.2.3.4", Port: 443, Username: "u"}
	b := ProxyNodeInfo{Type: "SOCKS5", Server: "1.2.3.4", Port: 443, Username: "u"}
	c := ProxyNodeInfo{Type: "socks5", Server: "1.2.3.4", Port: 443, Username: "v"}
	if proxyNodeKey(a) != proxyNodeKey(b) {
		t.Fatal("同节点不同大小写应视为相同")
	}
	if proxyNodeKey(a) == proxyNodeKey(c) {
		t.Fatal("不同用户名应视为不同节点")
	}
}

func TestShortSubscriptionLabel(t *testing.T) {
	got := shortSubscriptionLabel("https://subs.example.com/c/?token=abc")
	if got != "subs.example.com/c" {
		t.Fatalf("标签不对: %q", got)
	}
}

func TestFetchSubscriptionsMulti(t *testing.T) {
	mk := func(body string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(body))
		}))
	}
	enc := func(s string) string {
		return base64.StdEncoding.EncodeToString([]byte(s))
	}
	s1 := mk(enc("socks5://u1:p1@1.1.1.1:443#n1\nsocks5://u2:p2@2.2.2.2:443#n2"))
	defer s1.Close()
	s2 := mk(enc("socks5://u2:p2@2.2.2.2:443#n2-dup\nsocks5://u3:p3@3.3.3.3:443#n3"))
	defer s2.Close()
	bad := mk("not-a-subscription")
	defer bad.Close()

	urls := []string{s1.URL, s2.URL, bad.URL}
	fetched, errs := fetchSubscriptions(urls)
	if len(fetched) != 3 {
		t.Fatalf("返回分组数不对: %d", len(fetched))
	}
	if len(fetched[0]) != 2 || len(fetched[1]) != 2 {
		t.Fatalf("订阅节点数不对: %d / %d", len(fetched[0]), len(fetched[1]))
	}
	if len(errs) != 1 {
		t.Fatalf("失败订阅数不对: %d (%v)", len(errs), errs)
	}
	seen := map[string]bool{}
	total := 0
	for _, list := range fetched {
		for _, n := range list {
			k := proxyNodeKey(n)
			if seen[k] {
				continue
			}
			seen[k] = true
			total++
		}
	}
	if total != 3 {
		t.Fatalf("合并去重后应为 3，实际 %d", total)
	}
}
