//go:build !windows

package main

// focusCursorWindow 在非 Windows 平台上没有窗口可聚焦。
func focusCursorWindow(pid uint32) {}
