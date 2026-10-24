//go:build desktop && wails

package desktop

import (
	"log/slog"
	"path/filepath"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// RunShell Wails 原生外壳（-tags "desktop wails"），magpie 质感的窗口族：
//   - 主窗口 = 整个应用（教师台，含监考大屏 Tab），唯一常驻任务栏条目；
//   - 托盘面板 = 锚定在图标旁的速览小窗（不进任务栏，失焦即收）；
//   - 投屏窗口（扫码加入 / 监考大屏 / 学生预览）= 原生 WebView 窗口：
//     无地址栏、无浏览器外壳、Forge 图标——绝不落入系统浏览器；
//     按需弹出、常驻任务栏（拖去投影仪需要），关闭即隐藏。
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
	// 二次双击 = 唤起主窗（magpie 行为）。回调在 Options 里注册、
	// 窗口创建后赋值——通知只在 app.Run() 之后才可能到达，时序安全。
	var raiseMain func()
	app := application.New(application.Options{
		Name:        "Forge",
		Description: "Forge 竞赛工作台",
		Icon:        icon,
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: "forge.yviscool.app.desktop",
			OnSecondInstanceLaunch: func(d application.SecondInstanceData) {
				log.Info("second launch handed over to running instance", "args", d.Args)
				if raiseMain != nil {
					raiseMain()
				}
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
	// 启动显示用 magpie 的 Windows 信号：WebView 首次导航完成后再 Show——
	// 创建期/通用事件的显示时机在 beta.24 上不稳定（观察到偶发不显示）。
	mainShown := &sync.Once{}
	// 事件回调已在 UI 线程：直接 Show。包 InvokeSync 会嵌套同步（时好时坏的根因）。
	mainWin.OnWindowEvent(events.Windows.WebViewNavigationCompleted, func(*application.WindowEvent) {
		mainShown.Do(func() { mainWin.Show() })
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

	raiseMain = func() { application.InvokeSync(func() { mainWin.Show(); mainWin.Focus() }) }

	// 投屏窗口族：地址在创建期固定（SetURL 在 beta.24 上不可靠），
	// 打开 = Show + Focus；关闭 = 隐藏（下次打开回到本职页面）。
	joinWin := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name: "join", Title: "Forge · 学生扫码加入",
		URL: urls.Join, Width: 440, Height: 700, Hidden: true,
	})
	joinWin.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		e.Cancel()
		joinWin.Hide()
	})
	bigWin := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name: "bigscreen", Title: "Forge · 监考大屏",
		URL: urls.BigScreen, Width: 1280, Height: 800, Hidden: true,
	})
	bigWin.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		e.Cancel()
		bigWin.Hide()
	})
	studentWin := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name: "student", Title: "Forge · 学生大厅（预览）",
		URL: urls.Student, Width: 1100, Height: 760, Hidden: true,
	})
	studentWin.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		e.Cancel()
		studentWin.Hide()
	})

	reveal := func(win *application.WebviewWindow) {
		win.Show()
		win.Focus()
	}

	// UI 指令（面板/教师台按钮 → /ui/command）：
	// show-main 唤起主窗；投屏/预览类一律系统浏览器。
	// showMain = 显示主窗并回到教师台。主窗内允许自由切换分区（magpie 单窗
	// 语义），托盘/指令唤起时用 JS 导航回教师台——已在教师台则不刷新。
	showMain := func() {
		mainWin.Show()
		mainWin.Focus()
		mainWin.ExecJS(`if (!(location.pathname + '/').includes('/teacher')) location.replace('/teacher')`)
	}
	SetUICommandHandler(func(action string) {
		application.InvokeSync(func() {
			switch action {
			case "show-main":
				showMain()
			case "open-join":
				reveal(joinWin)
			case "open-bigscreen":
				reveal(bigWin)
			case "open-student":
				reveal(studentWin)
			}
		})
	})

	// 托盘菜单：主窗 + 投屏/预览 + 退出。
	tray := app.SystemTray.New()
	if len(icon) > 0 {
		tray.SetIcon(icon)
	}
	tray.SetTooltip("Forge 竞赛工作台")
	menu := app.NewMenu()
	menu.Add("打开教师台").OnClick(func(*application.Context) {
		application.InvokeSync(showMain)
	})
	menu.Add("学生扫码加入（投屏）").OnClick(func(*application.Context) {
		application.InvokeSync(func() { reveal(joinWin) })
	})
	menu.Add("监考大屏（投影）").OnClick(func(*application.Context) {
		application.InvokeSync(func() { reveal(bigWin) })
	})
	menu.Add("学生大厅（预览）").OnClick(func(*application.Context) {
		application.InvokeSync(func() { reveal(studentWin) })
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
