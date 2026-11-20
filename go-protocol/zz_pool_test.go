package main

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestParseClashFile(t *testing.T) {
	data, err := os.ReadFile(`C:\Users\dhl\Desktop\m3\singlebox-clash.yaml`)
	if err != nil {
		t.Skip("no local subscription file")
	}
	nodes, err := ParseSubscription(string(data))
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("nodes=%d", len(nodes))
	if len(nodes) < 100 {
		t.Fatalf("expected many nodes, got %d", len(nodes))
	}
}

func TestCheckFewNodes(t *testing.T) {
	data, err := os.ReadFile(`C:\Users\dhl\Desktop\m3\singlebox-clash.yaml`)
	if err != nil {
		t.Skip("no local subscription file")
	}
	nodes, _ := ParseSubscription(string(data))
	if len(nodes) > 12 {
		nodes = nodes[:12]
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	checked := CheckProxyNodes(ctx, nodes, 12, "https://api.ipify.org?format=json")
	alive := 0
	for _, n := range checked {
		t.Logf("%s %s:%d tls=%v alive=%v ip=%s %dms err=%s", n.Name, n.Server, n.Port, n.TLS, n.Alive, n.ExitIP, n.Latency, n.Error)
		if n.Alive {
			alive++
		}
	}
	t.Logf("alive=%d/%d", alive, len(checked))
}

