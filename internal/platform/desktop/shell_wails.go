//go:build desktop && wails

package desktop

import (
	"log/slog"
	"path/filepath"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// RunShell Wails 原生外壳（-tags "desktop wails"），与 magpie 同构：
//   - 主窗口 = 教师台（WebView 指向本机 HTTP 服务，SSE/Cookie/LAN 零改动）；
//   - 托盘左键单击 = 弹出锚定在图标旁的无边框速览面板（/app/panel），再点收起；
//   - 关闭主窗口 = 隐藏到托盘（不退出），退出走托盘菜单或 /shutdown API；
//   - 学生大厅 / 扫码页 / 监考大屏均为原生窗口，全程不经过浏览器；
//   - 单实例：二次启动唤起已有实例。
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
	// 就绪即显示主窗（窗口选项 Hidden:true 避免白帧，这里确定性亮出）。
	mainWin.Show()

	// 托盘速览面板：React 面板形态（/app/panel），无边框、置顶、失焦即收。
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

	// 学生扫码加入窗（/join 页自带说明与二维码，可常开投影）。
	joinWin := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name: "join", Title: "Forge · 学生扫码加入",
		URL: urls.Join, Width: 420, Height: 680, Hidden: true,
	})
	// 监考大屏窗（教师台直达监考分区，投屏用）。
	bigWin := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name: "bigscreen", Title: "Forge · 监考大屏",
		URL: urls.BigScreen, Width: 1280, Height: 800, Hidden: true,
	})
	// 学生大厅窗（教师预览学生视角，不占浏览器）。
	studentWin := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name: "student", Title: "Forge · 学生大厅",
		URL: urls.Student, Width: 1100, Height: 760, Hidden: true,
	})

	// 原生窗口指令执行器：面板/教师台的按钮经 /ui/command 到达这里。
	// 每次打开都先导航回该窗口的本职页面——用户在窗口里点到别处后，
	// 再点托盘/面板按钮必须回到预期视图，而不是停留在上次页面。
	showAt := func(win *application.WebviewWindow, url string) {
		win.SetURL(url)
		win.Show()
		win.Focus()
	}
	SetUICommandHandler(func(action string) {
		application.InvokeSync(func() {
			switch action {
			case "show-main":
				showAt(mainWin, urls.Teacher)
			case "open-join":
				showAt(joinWin, urls.Join)
			case "open-bigscreen":
				showAt(bigWin, urls.BigScreen)
			case "open-student":
				showAt(studentWin, urls.Student)
			}
		})
	})

	// 托盘菜单按考试日使用频率排序，全部指向原生窗口。
	tray := app.SystemTray.New()
	if len(icon) > 0 {
		tray.SetIcon(icon)
	}
	tray.SetTooltip("Forge 竞赛工作台")
	menu := app.NewMenu()
	menu.Add("监考面板（速览）").OnClick(func(*application.Context) {
		tray.ToggleWindow()
	})
	menu.Add("学生扫码加入").OnClick(func(*application.Context) {
		showAt(joinWin, urls.Join)
	})
	menu.Add("监考大屏（投屏）").OnClick(func(*application.Context) {
		showAt(bigWin, urls.BigScreen)
	})
	menu.AddSeparator()
	menu.Add("打开教师台").OnClick(func(*application.Context) {
		showAt(mainWin, urls.Teacher)
	})
	menu.Add("打开学生大厅").OnClick(func(*application.Context) {
		showAt(studentWin, urls.Student)
	})
	versionItem := menu.Add("Forge · 服务运行中")
	versionItem.SetEnabled(false)
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

	// /shutdown API 请求停机：收起托盘并退出（app.Quit 会触发 OnShutdown 收尾）。
	go func() {
		<-hooks.QuitRequested
		application.InvokeSync(func() {
			app.Quit()
		})
	}()

	log.Info("wails shell starting",
		"teacher", urls.Teacher, "panel", urls.Panel, "windows", 5)
	app.Run()
}
