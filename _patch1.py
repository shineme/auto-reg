import io, sys

def patch(path, old, new, count=1):
    with io.open(path, 'r', encoding='utf-8') as f:
        s = f.read()
    if old not in s:
        print('NOT FOUND in', path)
        print(repr(old[:120]))
        sys.exit(1)
    s = s.replace(old, new, count)
    with io.open(path, 'w', encoding='utf-8', newline='') as f:
        f.write(s)
    print('patched', path)

p = r'C:\Users\dhl\Desktop\m3\go-protocol\cursor.go'

patch(p,
"""\tif b.cancel != nil {
\t\tb.cancel()
\t}
}""",
"""\tif b.cancel != nil {
\t\tb.cancel()
\t}
\tif len(b.profileDir) > 0 {
\t\t_ = os.RemoveAll(b.profileDir)
\t}
}""")
