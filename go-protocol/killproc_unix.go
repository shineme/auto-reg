//go:build !windows

package main

import (
	"os"
	"syscall"
)

// killProcessTree 杀掉 Chrome 整个进程组（Linux 下以 Setpgid 启动）。
func killProcessTree(pid int) {
	_ = syscall.Kill(-pid, syscall.SIGKILL)
}

// chromeSysProcAttr 独立进程组，便于整树清理；root 运行需 --no-sandbox。
func chromeSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true}
}

var _ = os.Getenv
