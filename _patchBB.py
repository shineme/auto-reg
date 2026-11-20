import io
code = '''//go:build !windows

package main

// focusCursorWindow 在非 Windows 平台上没有窗口可聚焦。
func focusCursorWindow(pid uint32) {}
'''
io.open(r'C:\Users\dhl\Desktop\m3\go-protocol\cursor_focus_other.go','w',encoding='utf-8',newline='').write(code)
print('ok')
