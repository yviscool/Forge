package desktop

import "sync"

// UI 指令注册表：HTTP 层（/ui/command）把面板/教师台的按钮指令
// 转交给当前外壳。非 Wails 构建无执行器，端点返回 503、前端静默降级。
var (
	uiCmdMu   sync.Mutex
	uiCmdOnce sync.Once
	uiCmd     func(action string)
)

// SetUICommandHandler 注册原生窗口指令执行器（Wails 外壳启动时调用）。
func SetUICommandHandler(h func(action string)) {
	uiCmdMu.Lock()
	defer uiCmdMu.Unlock()
	uiCmdOnce.Do(func() { uiCmd = h })
}

// DispatchUICommand 由 HTTP 层调用：把面板/教师台的 UI 指令转给原生窗口。
func DispatchUICommand(action string) {
	uiCmdMu.Lock()
	h := uiCmd
	uiCmdMu.Unlock()
	if h != nil {
		h(action)
	}
}
