//go:build desktop && !wails

package desktop

import (
	"log/slog"

	"github.com/getlantern/systray"
)

// RunTray 系统托盘菜单（Wails 降级外壳用）。菜单按考试日使用频率排序：
// 学生扫码 > 监考大屏 > 全功能页 > 退出。
// Windows 纯 Go；Linux/Darwin 需 GTK/CGO，故仅在对应平台由调用方裁剪。
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

		openJoin := systray.AddMenuItem("学生扫码加入", "打开学生扫码加入页（可投屏）")
		openBigScreen := systray.AddMenuItem("监考大屏（投屏）", "在教师台打开全员监考大屏")
		systray.AddSeparator()
		openTeacher := systray.AddMenuItem("打开教师台", "在浏览器中打开教师台")
		openStudent := systray.AddMenuItem("打开学生大厅", "在浏览器中打开学生端")
		versionItem := systray.AddMenuItem("Forge · 服务运行中", "服务正在本机运行")
		versionItem.Disable()
		systray.AddSeparator()
		quit := systray.AddMenuItem("退出 Forge", "停止服务并退出")

		go func() {
			for {
				select {
				case <-openJoin.ClickedCh:
					if hooks.Browser != nil && urls.Join != "" {
						if err := hooks.Browser.Open(urls.Join); err != nil {
							log.Warn("open join page failed", "err", err)
						}
					}
				case <-openBigScreen.ClickedCh:
					if hooks.Browser != nil && urls.BigScreen != "" {
						if err := hooks.Browser.Open(urls.BigScreen); err != nil {
							log.Warn("open big screen failed", "err", err)
						}
					}
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
	onExit := func() {}
	systray.Run(onReady, onExit)
}
