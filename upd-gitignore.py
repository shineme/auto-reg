import io
p=".gitignore"
s=io.open(p,encoding="utf-8").read()
add="""
# Go protocol version (pure Go + Wails)
go-protocol/build/
go-protocol/artifacts/
go-protocol/*.exe
go-protocol/wailsjs/

# Local helper scripts / node data (may contain credentials)
vps-*.py
web-*-test.py
singlebox-*.yaml
singlebox-*.txt
cliproxy-*.html
cliproxy-*.txt
gen-singbox.py
"""
if "go-protocol/build/" not in s:
    s=s+add
io.open(p,"w",encoding="utf-8",newline="\n").write(s)
print("ok")
