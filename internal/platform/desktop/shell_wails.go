//go:build desktop && wails

package desktop

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// RunShell Wails 原生外壳（-tags "desktop wails"），magpie 质感的窗口族：
//   - 主窗口 = 整个应用（教师台 + 选手大厅等全部视图，窗内路由切换），
//     唯一常驻任务栏条目；
//   - 托盘面板 = 锚定在图标旁的速览小窗（不进任务栏，失焦即收）；
//   - 投屏窗口（扫码加入 / 监考大屏 / 学生预览）= 原生 WebView 窗口：
//     无地址栏、无浏览器外壳、Forge 图标；按需弹出，关闭即隐藏。
//   - 关闭主窗口 = 隐藏到托盘；退出走托盘菜单或 /shutdown API；单实例。
func RunShell(urls TrayURLs, hooks ShellHooks, log *slog.Logger) {
	icon, err := trayIcon()
	if err != nil {
		log.Warn("tray icon unavailable", "err", err)
		icon = nil
	}

	// 外壳诊断日志（beta.24 窗口时序问题的现场记录）。
	debugLog := func(format string, args ...any) {
		f, ferr := os.OpenFile(filepath.Join(".", "data", "shell-debug.log"),
			os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if ferr == nil {
			_, _ = fmt.Fprintf(f, format+"\n", args...)
			_ = f.Close()
		}
	}

	// 资产服务器代理：只服务 Wails 自身的运行时资源。业务页面一律直接加载
	// 真实 http 源（见下方 abs()），因为走资产代理会丢 POST body。
	backend, _ := url.Parse("http://127.0.0.1:8080")
	proxy := httputil.NewSingleHostReverseProxy(backend)
	proxy.FlushInterval = -1 // SSE 实时流
	assetHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxy.ServeHTTP(w, r)
	})

	// WebView2 用户数据目录放在 data/ 下：便携部署不在 %APPDATA% 留垃圾
	// （magpie appdir.WebView 同理）。
	profileDir := filepath.Join(".", "data", "webview2")
	// 二次双击 = 唤起主窗并回到教师台（magpie ShowMain 语义）。
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
		Assets: application.AssetOptions{
			Handler: assetHandler,
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

	// 页面加载策略（本轮实测结论，别再改回资产代理）：
	// 所有窗口直接加载真实服务地址 http://127.0.0.1:8080 —— 与局域网学生
	// 访问的是同一个进程、同一个源。三个必须的后果：
	//   1) POST 请求体完整。走 Wails 资产服务器（同源代理）时 beta.24 会丢
	//      body：dt 兑换会话与教师登录都返回 400 invalid json，主窗永远停在
	//      登录页、也永远登不进去；
	//   2) 主窗与托盘面板同源 → 面板能读到主窗用 ?dt= 换来的会话 cookie
	//      （面板若单独是别的源，就会一直显示「尚未登录教师账号」）；
	//   3) SSE 直连，不需要反向代理。
	// 资产代理只留给 Wails 自身运行时资源，不再承载业务页面。
	origin, perr := url.Parse(urls.Teacher)
	if perr != nil || origin.Scheme == "" || origin.Host == "" {
		origin, _ = url.Parse("http://127.0.0.1:8080")
	}
	abs := func(path string) string {
		if path == "" {
			return origin.Scheme + "://" + origin.Host
		}
		if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
			return path
		}
		return origin.Scheme + "://" + origin.Host + "/" + strings.TrimPrefix(path, "/")
	}

	// 主窗=教师台（可带 ?dt= 一次性登录令牌）；面板/投屏窗各自固定地址。
	mainURL := urls.MainURL
	if mainURL == "" {
		mainURL = abs("/teacher")
	}
	// 主窗口：教师台。不设 Hidden：创建期即显示（beta.24 事件驱动的显示
	// 时机不稳定，见下方兜底）。
	mainWin := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name: "main", Title: "Forge 教师台",
		URL:   mainURL,
		Width: 1280, Height: 820, MinWidth: 960, MinHeight: 640,
	})
	// 关闭 = 隐藏到托盘；退出走托盘菜单或 /shutdown API（magpie lightweight.go 同款）。
	mainWin.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		debugLog("[%s] WindowClosing[main] visible=%v", time.Now().Format("15:04:05.000"), mainWin.IsVisible())
		e.Cancel()
		mainWin.Hide()
	})
	// 启动显示兜底 1：WebView 首次导航完成后 Show（magpie 的 Windows 就绪信号）。
	// 回调已在 UI 线程，直接调用；包 InvokeSync 会嵌套同步。
	mainShown := &sync.Once{}
	mainWin.OnWindowEvent(events.Windows.WebViewNavigationCompleted, func(*application.WindowEvent) {
		mainShown.Do(func() {
			debugLog("boot show: navigation-completed")
			mainWin.Show()
		})
	})

	// 托盘速览面板：React 面板形态，无边框、置顶、失焦即收、不进任务栏
	// （WS_EX_TOOLWINDOW——避免出现第二个任务栏条目）。
	// 与主窗同源（同一个 http 服务地址）→ 共享会话 cookie，登录后自动出数据。
	// native=1 告诉面板：这是桌面外壳，动作走 /api/v1/ui/command 开原生窗口。
	panel := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name: "panel", Title: "Forge 监考速览",
		URL:   abs("/tray?native=1"),
		Width: 400, Height: 720,
		Hidden: true, Frameless: true, AlwaysOnTop: true,
		DisableResize: true, HideOnEscape: true, HideOnFocusLost: true,
		BackgroundType: application.BackgroundTypeTranslucent,
		Windows:        application.WindowsWindow{HiddenOnTaskbar: true},
	})
	panel.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		debugLog("[%s] WindowClosing[panel]", time.Now().Format("15:04:05.000"))
		e.Cancel()
		panel.Hide()
	})

	// 投屏窗口族：地址在创建期固定，打开 = Show + Focus；关闭 = 隐藏。
	joinWin := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name: "join", Title: "Forge · 学生扫码加入",
		URL: abs("/join"), Width: 440, Height: 700, Hidden: true,
	})
	joinWin.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		debugLog("[%s] WindowClosing[join]", time.Now().Format("15:04:05.000"))
		e.Cancel()
		joinWin.Hide()
	})
	bigWin := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name: "bigscreen", Title: "Forge · 监考大屏",
		URL: abs("/teacher?tab=monitor"), Width: 1280, Height: 800, Hidden: true,
	})
	bigWin.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		debugLog("[%s] WindowClosing[bigscreen]", time.Now().Format("15:04:05.000"))
		e.Cancel()
		bigWin.Hide()
	})
	studentWin := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name: "student", Title: "Forge · 学生大厅（预览）",
		URL: abs("/app"), Width: 1100, Height: 760, Hidden: true,
	})
	studentWin.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		debugLog("[%s] WindowClosing[student]", time.Now().Format("15:04:05.000"))
		e.Cancel()
		studentWin.Hide()
	})

	reveal := func(win *application.WebviewWindow) {
		win.Show()
		win.Focus()
	}

	// openMain = magpie 的 ShowMain/openMain + whenLoaded 语义（非阻塞版）：
	// SetURL 目标视图后，由「导航完成」事件负责 Show/Focus——导航过程中
	// WebView 会短暂隐藏窗口，立即 Show 会被盖掉（本轮实证）；在 UI 线程
	// 同步等待事件则会死锁，故用待显示标记 + 定时器兜底。
	var showPending atomic.Bool
	mainWin.OnWindowEvent(events.Windows.WebViewNavigationCompleted, func(*application.WindowEvent) {
		if showPending.CompareAndSwap(true, false) {
			mainWin.Show()
			mainWin.Focus()
		}
	})
	openMain := func(url string) {
		if url != "" {
			showPending.Store(true)
			mainWin.SetURL(abs(url))
			go func() {
				time.Sleep(2500 * time.Millisecond)
				if showPending.CompareAndSwap(true, false) {
					debugLog("openMain: nav-event timeout, fallback show")
					mainWin.Show()
					mainWin.Focus()
				}
			}()
			return
		}
		mainWin.Show()
		mainWin.Focus()
	}
	raiseMain = func() { application.InvokeSync(func() { openMain("/teacher") }) }

	SetUICommandHandler(func(action string) {
		debugLog("ui-command: %s", action)
		switch action {
		case "show-main":
			debugLog("openMain -> /teacher")
			openMain("/teacher")
			debugLog("openMain done")
		case "open-join":
			reveal(joinWin)
		case "open-bigscreen":
			reveal(bigWin)
		case "open-student":
			reveal(studentWin)
		}
	})

	// 托盘菜单：主窗 + 投屏 + 退出。
	tray := app.SystemTray.New()
	if len(icon) > 0 {
		tray.SetIcon(icon)
	}
	tray.SetTooltip("Forge 竞赛工作台")
	menu := app.NewMenu()
	menu.Add("打开教师台").OnClick(func(*application.Context) {
		application.InvokeSync(func() { openMain("/teacher") })
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

	// 启动显示兜底 2：事件循环就绪后由定时器显式 Show（各机制独立、幂等）。
	go func() {
		time.Sleep(4 * time.Second)
		debugLog("boot show: timer")
		mainWin.Show()
	}()

	log.Info("wails shell starting", "teacher", mainURL, "windows", "main+panel", "origin", abs("/"))
	app.Run()
}
