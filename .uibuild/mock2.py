import io
import os

ROOT = r"C:\Users\dhl\Desktop\m3\go-protocol\frontend"
ASSET = "file:///C:/Users/dhl/Desktop/m3/go-protocol/frontend/assets/"
OUT = r"C:\Users\dhl\Desktop\m3\.uibuild\mock2.html"

src = io.open(os.path.join(ROOT, "index.html"), encoding="utf-8").read()
src = src.replace('"/assets/', '"' + ASSET)
mock = io.open(r"C:\Users\dhl\Desktop\m3\.uibuild\mock2.js", encoding="utf-8").read()
out = src.replace("</body>", "<script>\n" + mock + "\n</script>\n</body>", 1)
io.open(OUT, "w", encoding="utf-8", newline="\n").write(out)
print("ok", OUT, len(out))
