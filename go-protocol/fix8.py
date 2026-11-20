import io
p="runner.go"
s=io.open(p,encoding="utf-8").read()
old="res := r.runOne(ctx, cfg, dialer, bridgeAddr, panel, ticketProvider, store, workerID, attempt)"
new="res := r.runOne(ctx, cfg, dialer, bridgeAddr, panel, ticketProvider, store, watcher, workerID, attempt)"
assert old in s
s=s.replace(old,new,1)
old2="func (r *ProtocolRunner) runOne(ctx context.Context, cfg Config, dialer *ProxyDialer, bridgeAddr string, panel *PanelClient, tp TicketProvider, store *HotmailPoolStore, workerID, attempt int) AccountResult {"
new2="func (r *ProtocolRunner) runOne(ctx context.Context, cfg Config, dialer *ProxyDialer, bridgeAddr string, panel *PanelClient, tp TicketProvider, store *HotmailPoolStore, watcher *poolWatcher, workerID, attempt int) AccountResult {"
assert old2 in s
s=s.replace(old2,new2,1)
old3="""	if dialer != nil {
		dialer.RotateSession()
	}"""
new3="""	// \u8282\u70b9\u6c60\u88ab\u5237\u65b0\u540e\u70ed\u52a0\u8f7d\uff08\u4e0d\u4e2d\u65ad\u6279\u91cf\uff09
	if watcher != nil {
		if specs, ok := watcher.refresh(); ok && len(specs) > 0 && dialer != nil {
			dialer.SetPoolNodes(specs, workerID-1)
			r.emitLog("info", fmt.Sprintf("[%d] \u8282\u70b9\u6c60\u5df2\u70ed\u66f4\u65b0\uff1a%s \u4e2a\u5b58\u6d3b\u8282\u70b9", workerID, len(specs)))
		}
	}
	if dialer != nil {
		dialer.RotateSession()
	}"""
assert old3 in s
s=s.replace(old3,new3,1)
io.open(p,"w",encoding="utf-8",newline="\n").write(s)
print("ok")
