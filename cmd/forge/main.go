package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/yviscool/forge/internal/adapters/store/memory"
	"github.com/yviscool/forge/internal/adapters/store/sqlite"
	"github.com/yviscool/forge/internal/app"
	"github.com/yviscool/forge/internal/auth"
	"github.com/yviscool/forge/internal/config"
	"github.com/yviscool/forge/internal/domain"
	"github.com/yviscool/forge/internal/judge"
	"github.com/yviscool/forge/internal/observability"
	"github.com/yviscool/forge/internal/platform/desktop"
	"github.com/yviscool/forge/internal/platform/diskfree"
	"github.com/yviscool/forge/internal/ports"
	"github.com/yviscool/forge/internal/realtime"
	"github.com/yviscool/forge/internal/testdata"
	httpapi "github.com/yviscool/forge/internal/transport/http"
)

var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func main() {
	// 桌面构建（-tags desktop）双击即桌面应用；--server 强制 headless。
	// 服务构建反之，--desktop 显式开启桌面体验。
	desktopMode := desktop.DefaultDesktopMode
	for _, arg := range os.Args[1:] {
		if arg == "--desktop" {
			desktopMode = true
		}
		if arg == "--server" {
			desktopMode = false
		}
		if arg == "--version" || arg == "version" {
			fmt.Printf("forge %s commit=%s built=%s\n", version, commit, buildDate)
			return
		}
	}
	hardenFileCreation()
	cfg := config.Load()
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	if err := cfg.Validate(); err != nil {
		log.Error("invalid configuration", "err", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(cfg.DataDir, 0o700); err != nil {
		log.Error("create data directory failed", "path", cfg.DataDir, "err", err)
		os.Exit(1)
	}
	if desktopMode {
		// windowsgui 构建没有控制台可看：日志同时写入 data/forge-desktop.log。
		logFile, err := os.OpenFile(filepath.Join(cfg.DataDir, "forge-desktop.log"),
			os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err == nil {
			defer logFile.Close()
			log = slog.New(slog.NewTextHandler(io.MultiWriter(os.Stdout, logFile), nil))
		}
	}

	// 手动 DI：按配置选择持久化后端（sqlite 文件默认，memory 按需）。
	var store ports.Store = memory.New()
	if cfg.Store == "sqlite" {
		db, err := sqlite.Open(cfg.DBPath)
		if err != nil {
			log.Error("open sqlite failed", "path", cfg.DBPath, "err", err)
			os.Exit(1)
		}
		defer db.Close()
		store = db
	}
	log.Info("store ready", "backend", cfg.Store)

	hub := realtime.NewHub()
	svc := app.NewService(store, hub, nil, log)
	srv := httpapi.NewServer(svc, log)
	testDataDir := filepath.Join(cfg.DataDir, "testdata")
	srv.Files = testdata.New(testDataDir)
	srv.AddReadinessCheck("data_dir", writableDirectoryCheck(cfg.DataDir))
	srv.AddReadinessCheck("test_data", srv.Files.Health)
	srv.AddReadinessCheck("disk_space", func(ctx context.Context) error {
		return diskfree.RequireAtLeast(
			ctx,
			cfg.DataDir,
			uint64(cfg.MinFreeDiskMiB)<<20,
		)
	})
	if err := judge.SetTestDataRoot(testDataDir); err != nil {
		log.Error("initialize test data root failed", "path", testDataDir, "err", err)
		os.Exit(1)
	}
	// 只有部署在可信反向代理之后时才解析代理头，避免考生伪造机位 IP。
	srv.TrustProxy = cfg.TrustProxy

	// 种子管理员（幂等；默认口令仅演示，生产必须改）。
	users, err := store.ListUsers()
	if err != nil {
		log.Error("inspect users failed", "err", err)
		os.Exit(1)
	}
	adminExists := false
	for _, u := range users {
		if u.Username == "admin" && u.Role == domain.RoleAdmin {
			adminExists = true
			break
		}
	}
	if _, err := auth.New(store, nil).EnsureAdmin(cfg.AdminPassword); err != nil {
		log.Error("ensure admin failed", "err", err)
		os.Exit(1)
	}
	if !adminExists && cfg.AdminPasswordGenerated {
		if err := os.MkdirAll(cfg.DataDir, 0o700); err != nil {
			log.Error("create bootstrap password directory failed", "err", err)
			os.Exit(1)
		}
		bootstrapPath := filepath.Join(cfg.DataDir, "bootstrap-admin-password.txt")
		if err := os.WriteFile(bootstrapPath, []byte(cfg.AdminPassword+"\n"), 0o600); err != nil {
			log.Error("write bootstrap password file failed", "path", bootstrapPath, "err", err)
			os.Exit(1)
		}
		log.Warn("bootstrap administrator created; read the one-time password from the protected file and delete it after rotating",
			"username", "admin", "path", bootstrapPath)
	}

	toolchainCfg, err := judge.ParseToolchainConfig(cfg.Toolchains)
	if err != nil {
		log.Error("invalid judge toolchain configuration", "err", err)
		os.Exit(1)
	}

	// 自动评测 worker：Submit → 入队 → JudgeOne → JudgeCases 回写。
	// 无测试点的题目跳过自动评测（保留教师手动判题）。
	workerCtx, stopWorkers := context.WithCancel(context.Background())
	go pruneExpiredSessions(workerCtx, store, log)
	var judgePool *judge.Pool
	if cfg.AutoJudge {
		log.Warn("UNSAFE local judge enabled: submitted code will run on this host",
			"protection", "process/memory walls only; no filesystem, network, or user isolation")
		tc := judge.Toolchain{Config: toolchainCfg}
		if cc, err := judge.NewCompileCache(filepath.Join(os.TempDir(), "forge-ccache")); err == nil {
			tc.Cache = cc
		}
		judgePool = judge.AutoJudgePool(svc, tc)
		judgePool.Start(workerCtx, cfg.JudgeWorkers)
		defer judgePool.Wait()
		srv.AddReadinessCheck("judge_workers", func(context.Context) error {
			return judgePool.Healthy()
		})
		srv.AddReadinessCheck("judge_toolchains", func(context.Context) error {
			return toolchainCfg.CheckCommands()
		})
		srv.AddReadinessCheck("judge_resource_walls", func(context.Context) error {
			return judge.CheckResourceLimitSupport()
		})
		enqueue := func(x domain.Submission) error {
			p, err := svc.GetProblem(x.ContestID, x.ProblemID)
			if err != nil {
				return err
			}
			req, ok := judge.BuildRequest(x, p)
			if !ok {
				return nil
			}
			if !judgePool.Submit(judge.Job{SubID: x.ID, Req: req}) {
				return judge.ErrQueueFull
			}
			return nil
		}
		srv.OnSubmit = enqueue
		go judge.RunQueueReconciler(
			workerCtx,
			svc,
			judgePool,
			judge.DefaultReconcileInterval,
			log,
		)
		log.Info("autojudge ready", "workers", cfg.JudgeWorkers)
	}
	srv.SetPresenceSource(func(cid string) map[string]time.Time {
		presence := hub.PresenceMap(cid)
		out := make(map[string]time.Time, len(presence))
		for uid, p := range presence {
			out[uid] = p.Since
		}
		return out
	})
	srv.SetRuntimeStatsProvider(func() observability.RuntimeStats {
		stats := observability.RuntimeStats{SSESubscriberCount: hub.SubscriberCount()}
		if judgePool != nil {
			judgeStats := judgePool.Stats()
			stats.JudgeWorkers = judgeStats.Workers
			stats.JudgeJobsActive = judgeStats.Active
			stats.JudgeJobsQueued = judgeStats.Queued
			stats.JudgeQueueCapacity = judgeStats.Capacity
		}
		return stats
	})

	httpSrv := &http.Server{
		Addr: cfg.Addr,
		// /join 学生扫码页（免登录）挂在主服务之前；其余请求原样透传。
		Handler:           desktop.JoinPageHandler(srv, cfg.Addr),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       5 * time.Minute,
		IdleTimeout:       2 * time.Minute,
		MaxHeaderBytes:    1 << 20,
		// WriteTimeout intentionally remains zero: SSE responses are long-lived.
	}
	go func() {
		log.Info("forge starting", "addr", cfg.Addr, "environment", cfg.Environment)
		log.Info("app", "url", localAppURL(cfg.Addr))
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("listen failed", "err", err)
			os.Exit(1)
		}
	}()

	if desktopMode {
		quitRequested := make(chan struct{})
		srv.SetShutdownHook(func() { close(quitRequested) })
		srv.SetUICommander(desktop.DispatchUICommand)
		runDesktopShell(cfg, log, httpSrv, stopWorkers, quitRequested)
		return
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	stopWorkers()
	ctx2, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(ctx2)
	log.Info("forge stopped")
}

func writableDirectoryCheck(dir string) func(context.Context) error {
	return func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
		f, err := os.CreateTemp(dir, ".forge-ready-*")
		if err != nil {
			return err
		}
		name := f.Name()
		defer os.Remove(name)
		if err := os.Chmod(name, 0o600); err != nil {
			_ = f.Close()
			return err
		}
		if _, err := f.Write([]byte("ok")); err != nil {
			_ = f.Close()
			return err
		}
		if err := f.Sync(); err != nil {
			_ = f.Close()
			return err
		}
		return f.Close()
	}
}

func localAppURL(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "/app"
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "localhost"
	}
	return "http://" + net.JoinHostPort(host, port) + "/app"
}

// runDesktopShell 教师机桌面模式：等服务就绪 → 打印加入卡片（含扫码二维码）→
// 进入桌面外壳并自动打开教师台。
// 外壳按构建标签选择：-tags "desktop wails" 为 Wails 原生窗口（主窗 + 托盘
// 锚定速览面板 + 单实例）；否则为托盘菜单 + 浏览器（纯 Go，无 CGO）。
// /shutdown API（托盘速览面板「退出服务」）与托盘退出都走同一收尾。
func runDesktopShell(cfg config.Config, log *slog.Logger, httpSrv *http.Server, stopWorkers func(), quitRequested chan struct{}) {
	healthz := desktop.LocalURL(cfg.Addr, "/healthz")
	if !desktop.WaitForReady(healthz, 10*time.Second) {
		log.Warn("desktop shell: health probe not ready; continuing without auto-open")
	}
	joinURLs := desktop.LANURLs(cfg.Addr, "/app")
	_ = desktop.PrintJoinCard(os.Stdout, desktop.LANURLs(cfg.Addr, ""), "/app")
	teacherURL := desktop.LocalURL(cfg.Addr, "/teacher")
	log.Info("desktop shell ready", "teacher", teacherURL)

	browser := desktop.SystemBrowser{}
	if desktop.WailsShellEnabled() {
		// Wails 原生壳的主窗即教师台，不再另开浏览器标签。
		log.Info("native window will host the teacher console")
	} else if err := browser.Open(teacherURL); err != nil {
		log.Warn("auto-open teacher console failed; please open the URL manually", "url", teacherURL, "err", err)
	}

	trayURLs := desktop.TrayURLs{
		Teacher:   teacherURL,
		Student:   desktop.LocalURL(cfg.Addr, "/app"),
		StudentQR: firstLANURL(cfg.Addr),
		LAN:       joinURLs,
		Join:      desktop.LocalURL(cfg.Addr, "/join"),
		Panel:     desktop.LocalURL(cfg.Addr, "/tray"),
		BigScreen: desktop.LocalURL(cfg.Addr, "/teacher?tab=monitor"),
	}

	desktop.RunShell(trayURLs, desktop.ShellHooks{
		Browser:       browser,
		QuitRequested: quitRequested,
		OnQuit: func() {
			stopWorkers()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = httpSrv.Shutdown(ctx)
			log.Info("forge stopped (desktop)")
		},
	}, log)
}

func firstLANURL(listenAddr string) string {
	urls := desktop.LANURLs(listenAddr, "/app")
	if len(urls) == 0 {
		return desktop.LocalURL(listenAddr, "/app")
	}
	return urls[0]
}

func pruneExpiredSessions(ctx context.Context, store ports.Store, log *slog.Logger) {
	prune := func() {
		deleted, err := store.DeleteExpiredSessions(time.Now().UTC())
		if err != nil {
			log.Error("prune expired sessions failed", "err", err)
			return
		}
		if deleted > 0 {
			log.Info("expired sessions pruned", "count", deleted)
		}
	}
	prune()
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			prune()
		}
	}
}
