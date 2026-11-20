package main

import "strings"

// isRecoverableNetworkError 判断错误是否属于「换个出口就能好」的网络类问题。
// 代理节点不响应、TLS 被 reset、连接超时都算，换 IP 重试有意义。
func isRecoverableNetworkError(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	marks := []string{
		"timeout",
		"timed out",
		"i/o timeout",
		"connection reset",
		"connection refused",
		"connection closed",
		"eof",
		"broken pipe",
		"reset by peer",
		"proxy",
		"tunnel",
		"tls handshake",
		"no such host",
		"context deadline exceeded",
	}
	for _, m := range marks {
		if strings.Contains(s, m) {
			return true
		}
	}
	return false
}
