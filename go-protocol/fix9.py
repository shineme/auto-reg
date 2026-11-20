import io
p="runner.go"
s=io.open(p,encoding="utf-8").read()
s=s.replace('r.emitLog("info", fmt.Sprintf("[%d] \u8282\u70b9\u6c60\u5df2\u70ed\u66f4\u65b0\uff1a%s \u4e2a\u5b58\u6d3b\u8282\u70b9", workerID, len(specs)))','r.emitLog("info", fmt.Sprintf("[%d] \u8282\u70b9\u6c60\u5df2\u70ed\u66f4\u65b0\uff1a%d \u4e2a\u5b58\u6d3b\u8282\u70b9", workerID, len(specs)))')
io.open(p,"w",encoding="utf-8",newline="\n").write(s)
print("ok")
