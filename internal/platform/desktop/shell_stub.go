//go:build !desktop

package desktop

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

// RunShell 非 desktop 构建的空实现：desktop 分支的代码要能编译，
// 但永远不会被调用（main 只在 desktopMode 时进入）。
func RunShell(_ TrayURLs, hooks ShellHooks, _ *slog.Logger) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-hooks.QuitRequested:
	case <-quit:
	}
}
