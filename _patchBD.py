import io
p = r'C:\Users\dhl\Desktop\m3\go-protocol\cursor_focus_windows.go'
s = io.open(p, encoding='utf-8').read()
old = '\tprocEnumWindows.Call(func(hwnd uintptr, lp uintptr) uintptr {'
new = '\tcb := syscall.NewCallback(func(hwnd uintptr, lp uintptr) uintptr {'
assert old in s
s = s.replace(old, new, 1)
old2 = '\t\treturn 0\n\t}, 0, 0)'
new2 = '\t\treturn 0\n\t})\n\tprocEnumWindows.Call(cb, 0, 0)'
assert old2 in s
io.open(p, 'w', encoding='utf-8', newline='').write(s.replace(old2, new2, 1))
print('ok')
