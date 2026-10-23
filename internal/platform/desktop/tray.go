//go:build desktop

package desktop

import (
	"fmt"
	"log/slog"
	"sort"

	"github.com/getlantern/systray"
)

// RunTray 启动系统托盘（阻塞直至退出）。菜单：
// 打开教师台 / 打开学生大厅 / 复制局域网地址（逐条列出）/ 退出。
// 仅在 --desktop 模式调用；CI 与 headless 环境不进入此路径。
func RunTray(urls TrayURLs, hooks TrayHooks, log *slog.Logger) {
	icon, err := trayIcon()
	if err != nil {
		log.Warn("tray icon unavailable", "err", err)
		icon = nil
	}
	onReady := func() {
		if len(icon) > 0 {
			systray.SetIcon(icon)
		}
		systray.SetTitle("Forge")
		systray.SetTooltip("Forge 竞赛工作台")

		openTeacher := systray.AddMenuItem("打开教师台", "在浏览器中打开教师台")
		openStudent := systray.AddMenuItem("打开学生大厅", "在浏览器中打开学生端")
		systray.AddSeparator()
		quit := systray.AddMenuItem("退出 Forge", "停止服务并退出")

		openTeacher.Enable()
		openStudent.Enable()
		go func() {
			for {
				select {
				case <-openTeacher.ClickedCh:
					if hooks.Browser != nil && urls.Teacher != "" {
						if err := hooks.Browser.Open(urls.Teacher); err != nil {
							log.Warn("open teacher console failed", "err", err)
						}
					}
				case <-openStudent.ClickedCh:
					if hooks.Browser != nil && urls.Student != "" {
						if err := hooks.Browser.Open(urls.Student); err != nil {
							log.Warn("open student lobby failed", "err", err)
						}
					}
				case <-quit.ClickedCh:
					systray.Quit()
				}
			}
		}()
	}
	onExit := func() {
		if hooks.OnQuit != nil {
			hooks.OnQuit()
		}
	}
	// 托盘退出后打印一次地址清单（便于教师从日志取链接）。
	go logLANURLs(urls.LAN, log)
	systray.Run(onReady, onExit)
}

func logLANURLs(lan []string, log *slog.Logger) {
	if len(lan) == 0 {
		return
	}
	sort.Strings(lan)
	for _, u := range lan {
		log.Info("students can join via", "url", fmt.Sprintf("%s/app", u))
	}
}
