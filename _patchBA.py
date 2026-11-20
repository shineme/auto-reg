import io
code = '''//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

var (
	user32                    = syscall.NewLazyDLL("user32.dll")
	procSetForegroundWindow   = user32.NewProc("SetForegroundWindow")
	procShowWindow            = user32.NewProc("ShowWindow")
	procEnumWindows           = user32.NewProc("EnumWindows")
	procGetWindowThreadProcID = user32.NewProc("GetWindowThreadProcessId")
)

// focusCursorWindow 把指定进程的 Chrome 窗口带到前台，方便用户手动点人机验证。
func focusCursorWindow(pid uint32) {
	if pid == 0 {
		return
	}
	procEnumWindows.Call(func(hwnd uintptr, lp uintptr) uintptr {
		var wpid uint32
		procGetWindowThreadProcID.Call(hwnd, uintptr(unsafe.Pointer(&wpid)))
		if wpid != pid {
			return 1
		}
		procShowWindow.Call(hwnd, 9)
		procSetForegroundWindow.Call(hwnd)
		return 0
	}, 0, 0)
}
'''
io.open(r'C:\Users\dhl\Desktop\m3\go-protocol\cursor_focus_windows.go','w',encoding='utf-8',newline='').write(code)
print('ok')
