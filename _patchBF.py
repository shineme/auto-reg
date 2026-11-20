import io
code = '''package main

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"
)

// cursorRunState 记录 Cursor 绑定任务的状态。
var cursorRunState struct {
	mu      sync.Mutex
	running bool
	cancel  context.CancelFunc
}
'''
io.open(r'C:\\Users\\dhl\\Desktop\\m3\\go-protocol\\cursor_app.go','w',encoding='utf-8',newline='').write(code)
print('ok')
