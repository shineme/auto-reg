package main

// MarkCurrentNodeDead 把当前节点从池里摘掉（取票据超时等场景）。
// 摘掉后 RotateSession 不会再轮到它，避免反复卡在同一个坏出口上。
func (p *ProxyDialer) MarkCurrentNodeDead() string {
	if p == nil {
		return ""
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.nodes) == 0 {
		return ""
	}
	idx := p.nodeIdx
	if idx < 0 || idx >= len(p.nodes) {
		return ""
	}
	name := p.nodes[idx].Name
	// 从切片里移除当前节点。
	p.nodes = append(p.nodes[:idx], p.nodes[idx+1:]...)
	if p.nodeIdx >= len(p.nodes) {
	p.nodeIdx = 0
}
	return name
}
