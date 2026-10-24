//go:build desktop && !wails

package desktop

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

// RunShell 无 Wails 的降级外壳（-tags desktop 不带 wails）：
// 系统托盘菜单 + 自动开浏览器。Windows 纯 Go 无 CGO；
// Linux/Darwin 的托盘需要 GTK/CGO，因此仅 Windows 载入真实托盘，
// 其余平台退化为「Ctrl+C 退出」+ 控制台加入卡片。
func RunShell(urls TrayURLs, hooks ShellHooks, log *slog.Logger) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	RunTray(urls, TrayHooks{Browser: hooks.Browser}, log)
	// RunTray 返回即托盘退出；再等停机请求或信号收尾。
	select {
	case <-hooks.QuitRequested:
	case <-quit:
	}
}

// WaitForQuitRequest 兼容入口：见 RunShell。
func WaitForQuitRequest(hooks ShellHooks) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-hooks.QuitRequested:
	case <-quit:
	}
}
