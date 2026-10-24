//go:build desktop && wails

package desktop

import (
	"log/slog"
	"path/filepath"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// RunShell Wails 原生外壳（-tags "desktop wails"）：
//   - 主窗口 = 教师台（WebView 指向本机 HTTP 服务，SSE/Cookie/LAN 零改动）；
//   - 托盘左键单击 = 弹出锚定在图标旁的无边框速览面板（/tray），再点收起；
//   - 关闭主窗口 = 隐藏到托盘（不退出），退出走托盘菜单或 /shutdown API；
//   - 单实例：二次启动唤起已有窗口。
//
// 与 magpie 同构（internal/gui/app.go），仅保留考场场景需要的子集。
func RunShell(urls TrayURLs, hooks ShellHooks, log *slog.Logger) {
	icon, err := trayIcon()
	if err != nil {
		log.Warn("tray icon unavailable", "err", err)
		icon = nil
	}

	// WebView2 的用户数据目录放在 data/ 下：便携部署不在 %APPDATA% 留垃圾
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

	// 主窗口：教师台。加载本机 HTTP 服务（而非资产协议），
	// 保证 SSE、Cookie 与学生端走同一条真实链路。
	mainWin := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:     "main",
		Title:    "Forge 教师台",
		URL:      urls.Teacher,
		Width:    1280,
		Height:   820,
		MinWidth: 960, MinHeight: 640,
		Hidden: true, // 就绪后再显示，避免白帧
	})
	// 关闭 = 隐藏到托盘；退出走托盘菜单或 /shutdown API（magpie lightweight.go:73）。
	mainWin.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		e.Cancel()
		mainWin.Hide()
	})

	// 托盘速览面板：无边框、置顶、失焦即收（magpie panelOptions 同款）。
	panel := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:            "panel",
		Title:           "Forge 监考速览",
		URL:             urls.Panel,
		Width:           400,
		Height:          720,
		Hidden:          true,
		Frameless:       true,
		AlwaysOnTop:     true,
		DisableResize:   true,
		HideOnEscape:    true,
		HideOnFocusLost: true,
		BackgroundType:  application.BackgroundTypeTranslucent,
	})
	panel.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		e.Cancel()
		panel.Hide()
	})

	// 托盘。
	tray := app.SystemTray.New()
	if len(icon) > 0 {
		tray.SetIcon(icon)
	}
	tray.SetTooltip("Forge 竞赛工作台")
	menu := app.NewMenu()
	openPanel := menu.Add("监考面板（速览）").OnClick(func(*application.Context) {
		tray.ToggleWindow()
	})
	openJoin := menu.Add("学生扫码加入").OnClick(func(*application.Context) {
		if hooks.Browser != nil {
			_ = hooks.Browser.Open(urls.Join)
		}
	})
	menu.AddSeparator()
	openTeacher := menu.Add("打开教师台").OnClick(func(*application.Context) {
		mainWin.Show()
		mainWin.Focus()
	})
	openStudent := menu.Add("打开学生大厅（浏览器）").OnClick(func(*application.Context) {
		if hooks.Browser != nil {
			_ = hooks.Browser.Open(urls.Student)
		}
	})
	versionItem := menu.Add("Forge · 服务运行中")
	versionItem.SetEnabled(false)
	_ = versionItem
	menu.AddSeparator()
	menu.Add("退出 Forge").OnClick(func(*application.Context) {
		app.Quit()
	})
	tray.SetMenu(menu)
	// 面板锚定在托盘图标旁（左键弹出/收起的核心）。
	tray.AttachWindow(panel).WindowOffset(6)
	tray.OnClick(func() {
		tray.ToggleWindow()
	})
	_ = openPanel
	_ = openJoin
	_ = openTeacher
	_ = openStudent

	// /shutdown API 请求停机：收起托盘并退出（app.Quit 会触发 OnShutdown 收尾）。
	go func() {
		<-hooks.QuitRequested
		application.InvokeSync(func() {
			app.Quit()
		})
	}()

	log.Info("wails shell starting", "teacher", urls.Teacher, "panel", urls.Panel)
	app.Run()
}
