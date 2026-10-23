//go:build !desktop

package desktop

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

// RunTray 降级实现：未以 `-tags desktop` 构建时不加载托盘
// （Linux/Darwin 托盘需要 CGO/GTK），桌面模式退化为
// 自动开浏览器 + 控制台加入卡片，Ctrl+C 与托盘退出等价。
func RunTray(urls TrayURLs, hooks TrayHooks, log *slog.Logger) {
	log.Warn("this build has no system tray (rebuild with -tags desktop); press Ctrl+C to stop",
		"teacher", urls.Teacher, "student", urls.Student)
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	if hooks.OnQuit != nil {
		hooks.OnQuit()
	}
}
