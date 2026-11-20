package main

import (
	"os"
	"testing"
)

// TestParseSubscriptionLive 用真实订阅文件验证解析（需设置 MINIMAX_SUB_FILE）。
func TestParseSubscriptionLive(t *testing.T) {
	path := os.Getenv("MINIMAX_SUB_FILE")
	if path == "" {
		t.Skip("未设置 MINIMAX_SUB_FILE")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取失败: %v", err)
	}
	nodes, err := ParseSubscription(string(data))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	t.Logf("解析到 %d 个节点", len(nodes))
	for i, n := range nodes {
		if i >= 5 {
			break
		}
		t.Logf("  %s | %s %s:%d user=%s", n.Name, n.Type, n.Server, n.Port, n.Username)
	}
	if len(nodes) == 0 {
		t.Fatal("没有解析到任何节点")
	}
}
