//go:build desktop && wails

package desktop

import (
	"log/slog"
	"path/filepath"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// RunShell Wails 原生外壳（-tags "desktop wails"），与 magpie 相同的双面模型：
//   - 主窗口 = 整个应用（教师台，含监考大屏 Tab）。唯一的任务栏条目。
//   - 托盘面板 = 锚定在图标旁的速览小窗（不进任务栏，失焦即收）。
//   - 面向「别人」的页面（学生扫码投屏、监考大屏投影、学生端预览）
//     一律走系统浏览器——它们本来就是要投到别的屏幕/设备上的。
//   - 关闭主窗口 = 隐藏到托盘；退出走托盘菜单或 /shutdown API；单实例。
func RunShell(urls TrayURLs, hooks ShellHooks, log *slog.Logger) {
	icon, err := trayIcon()
	if err != nil {
		log.Warn("tray icon unavailable", "err", err)
		icon = nil
	}

	// WebView2 用户数据目录放在 data/ 下：便携部署不在 %APPDATA% 留垃圾
	// （magpie appdir.WebView 同理）。
	profileDir := filepath.Join(".", "data", "webview2")
	app := application.New(application.Options{
		Name:        "Forge",
		Description: "Forge 竞赛工作台",
		Icon:        icon,
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: "forge.yviscool.app.desktop",
			OnSecondInstanceLaunch: func(d application.SecondInstanceData) {
				log.Info("second launch handed over to running instance", "args", d.Args)
			},
		},
		Windows: application.WindowsOptions{
			DisableQuitOnLastWindowClosed: true,
			WebviewUserDataPath:           profileDir,
		},
		ErrorHandler: func(err error) { log.Error("wails:", "err", err) },
		OnShutdown: func() {
			if hooks.OnQuit != nil {
				hooks.OnQuit()
			}
		},
	})

	mainURL := urls.MainURL
	if mainURL == "" {
		mainURL = urls.Teacher
	}
	// 主窗口：教师台。加载本机 HTTP 服务（非资产协议），SSE/Cookie 与学生端同链路。
	// 不设 Hidden：创建期 Show 会被未启动的事件循环丢弃，让窗口原生显示。
	mainWin := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name: "main", Title: "Forge 教师台",
		URL:   mainURL,
		Width: 1280, Height: 820, MinWidth: 960, MinHeight: 640,
	})
	// 关闭 = 隐藏到托盘；退出走托盘菜单或 /shutdown API（magpie lightweight.go 同款）。
	mainWin.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		e.Cancel()
		mainWin.Hide()
	})

	// 托盘速览面板：React 面板形态（/app/panel），无边框、置顶、失焦即收、
	// 不进任务栏（WS_EX_TOOLWINDOW——避免出现第二个任务栏条目）。
	panel := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name: "panel", Title: "Forge 监考速览",
		URL:   urls.Panel,
		Width: 400, Height: 720,
		Hidden: true, Frameless: true, AlwaysOnTop: true,
		DisableResize: true, HideOnEscape: true, HideOnFocusLost: true,
		BackgroundType: application.BackgroundTypeTranslucent,
		Windows:        application.WindowsWindow{HiddenOnTaskbar: true},
	})
	panel.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		e.Cancel()
		panel.Hide()
	})

	openExternal := func(url string) {
		if hooks.Browser == nil || url == "" {
			return
		}
		if err := hooks.Browser.Open(url); err != nil {
			log.Warn("open external surface failed", "url", url, "err", err)
		}
	}

	// UI 指令（面板/教师台按钮 → /ui/command）：
	// show-main 唤起主窗；投屏/预览类一律系统浏览器。
	SetUICommandHandler(func(action string) {
		application.InvokeSync(func() {
			switch action {
			case "show-main":
				mainWin.Show()
				mainWin.Focus()
			case "open-join":
				openExternal(urls.Join)
			case "open-bigscreen":
				openExternal(urls.BigScreen)
			case "open-student":
				openExternal(urls.Student)
			}
		})
	})

	// 托盘菜单：主窗 + 投屏/预览（浏览器）+ 退出。
	tray := app.SystemTray.New()
	if len(icon) > 0 {
		tray.SetIcon(icon)
	}
	tray.SetTooltip("Forge 竞赛工作台")
	menu := app.NewMenu()
	menu.Add("打开教师台").OnClick(func(*application.Context) {
		application.InvokeSync(func() {
			mainWin.Show()
			mainWin.Focus()
		})
	})
	menu.Add("学生扫码加入（投屏）").OnClick(func(*application.Context) {
		openExternal(urls.Join)
	})
	menu.Add("监考大屏（投影）").OnClick(func(*application.Context) {
		openExternal(urls.BigScreen)
	})
	menu.Add("学生大厅（预览）").OnClick(func(*application.Context) {
		openExternal(urls.Student)
	})
	menu.AddSeparator()
	versionItem := menu.Add("Forge · 服务运行中")
	versionItem.SetEnabled(false)
	menu.AddSeparator()
	menu.Add("退出 Forge").OnClick(func(*application.Context) {
		app.Quit()
	})
	tray.SetMenu(menu)
	// 面板锚定在托盘图标旁（左键弹出/收起）。
	tray.AttachWindow(panel).WindowOffset(6)
	tray.OnClick(func() {
		tray.ToggleWindow()
	})

	// /shutdown API 请求停机：退出应用（OnShutdown 负责优雅收尾）。
	go func() {
		<-hooks.QuitRequested
		application.InvokeSync(func() {
			app.Quit()
		})
	}()

	log.Info("wails shell starting", "teacher", mainURL, "panel", urls.Panel, "windows", "main+panel")
	app.Run()
}
