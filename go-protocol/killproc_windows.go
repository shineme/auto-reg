//go:build windows

package main

import (
	"os/exec"
	"strconv"
	"syscall"
)

// killProcessTree 杀掉 Chrome 进程树（Windows：taskkill /T）。
func killProcessTree(pid int) {
	_ = exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/T", "/F").Run()
}

// chromeSysProcAttr Windows 下无需额外进程组设置。
func chromeSysProcAttr() *syscall.SysProcAttr {
	return nil
}
