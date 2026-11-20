package main

import "testing"

func TestClipProxyUsername(t *testing.T) {
	p, err := NewProxyDialer("socks5://user-region-US:pass@us.cliproxy.io:10000")
	if err != nil {
		t.Fatal(err)
	}
	p.SetOptions([]string{"US", "JP"}, 10, StyleClipProxy, "Louisiana", true)
	p.RotateSession()
	u := p.stickyUser()
	t.Logf("username=%s", u)
	if !contains(u, "-sid-") || !contains(u, "-t-10") || !contains(u, "-st-Louisiana") {
		t.Fatalf("clip proxy username wrong: %s", u)
	}
	if !contains(u, "region-JP") {
		t.Fatalf("region missing: %s", u)
	}
	p.RotateSession()
	if p.stickyUser() == u {
		t.Fatal("sid should rotate")
	}
	// rotating mode
	p2, _ := NewProxyDialer("socks5://user-region-US:pass@us.cliproxy.io:10000")
	p2.SetOptions(nil, 0, StyleClipProxy, "", false)
	p2.RotateSession()
	if contains(p2.stickyUser(), "-sid-") {
		t.Fatalf("rotating should not have sid: %s", p2.stickyUser())
	}
	// 711 style
	p3, _ := NewProxyDialer("socks5://USER-zone-custom:pw@global.rotgb.trustsource.live:10000")
	p3.SetOptions([]string{"JP"}, 15, Style711, "", true)
	p3.RotateSession()
	t.Logf("711=%s", p3.stickyUser())
	if !contains(p3.stickyUser(), "-session-") || !contains(p3.stickyUser(), "-sessTime-15") {
		t.Fatalf("711 username wrong: %s", p3.stickyUser())
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
