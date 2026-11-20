package main

import "sync"

// EventSink 是事件发射的抽象：桌面版接 Wails EventsEmit，Web 版接 WebSocket 广播。
type EventSink func(event string, data any)

var (
	eventSinkMu sync.RWMutex
	eventSinkFn EventSink
)

func SetEventSink(fn EventSink) {
	eventSinkMu.Lock()
	eventSinkFn = fn
	eventSinkMu.Unlock()
}

func emitEvent(event string, data any) {
	eventSinkMu.RLock()
	fn := eventSinkFn
	eventSinkMu.RUnlock()
	if fn != nil {
		fn(event, data)
	}
}
