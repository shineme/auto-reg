import io, sys
def patch(path, old, new):
    s = io.open(path, encoding='utf-8').read()
    if old not in s:
        print('NOT FOUND'); print(repr(old[:120])); sys.exit(1)
    io.open(path, 'w', encoding='utf-8', newline='').write(s.replace(old, new, 1))
    print('ok')
p = r'C:\Users\dhl\Desktop\m3\go-protocol\cursor.go'
patch(p, '\t\t"--window-position=-32000,-32000",\n', '\t')
