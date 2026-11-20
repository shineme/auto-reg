import json,urllib.request
req=urllib.request.Request("https://api.github.com/repos/SagerNet/sing-box/releases/latest",headers={"User-Agent":"x"})
d=json.load(urllib.request.urlopen(req,timeout=30))
print(d["tag_name"])
for a in d["assets"]:
    if "linux-amd64" in a["name"] and a["name"].endswith(".tar.gz"):
        print(a["name"], a["browser_download_url"])
