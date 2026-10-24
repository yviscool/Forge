package desktop

// ShellHooks 桌面外壳与主程序的对接点：打开浏览器、优雅停机、
// /shutdown API（托盘面板「退出服务」）触发后的收尾通知。
// 所有构建组合可见：main 的 desktop 分支在任意标签下都要能编译。
type ShellHooks struct {
	Browser BrowserOpener
	// QuitRequested 在收到停机请求（/shutdown API）时被关闭。
	QuitRequested <-chan struct{}
	// OnQuit 外壳退出后的收尾（停 worker、停 HTTP）；Wails 外壳经 OnShutdown 调用。
	OnQuit func()
}
