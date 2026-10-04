package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/yviscool/forge/internal/adapters/store/memory"
	"github.com/yviscool/forge/internal/adapters/store/sqlite"
	"github.com/yviscool/forge/internal/app"
	"github.com/yviscool/forge/internal/auth"
	"github.com/yviscool/forge/internal/config"
	"github.com/yviscool/forge/internal/domain"
	"github.com/yviscool/forge/internal/judge"
	"github.com/yviscool/forge/internal/ports"
	"github.com/yviscool/forge/internal/realtime"
	httpapi "github.com/yviscool/forge/internal/transport/http"
)

func main() {
	cfg := config.Load()
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))

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

	svc := app.NewService(store, realtime.NewHub(), nil, log)
	srv := httpapi.NewServer(svc, log)

	// 种子管理员（幂等；默认口令仅演示，生产必须改）。
	if _, err := auth.New(store, nil).EnsureAdmin(cfg.AdminPassword); err != nil {
		log.Error("ensure admin failed", "err", err)
		os.Exit(1)
	}
	if cfg.AdminPassword == "admin123" {
		log.Warn("default admin password in use; set FORGE_ADMIN_PASSWORD immediately")
	}

	// 自动评测 worker：Submit → 入队 → JudgeOne → JudgeCases 回写。
	// 无测试点的题目跳过自动评测（保留教师手动判题）。
	workerCtx, stopWorkers := context.WithCancel(context.Background())
	if cfg.AutoJudge {
		pool := judge.AutoJudgePool(svc, runtime.NumCPU())
		pool.Start(workerCtx, runtime.NumCPU())
		defer pool.Wait()
		srv.OnSubmit = func(x domain.Submission) {
			p, err := svc.GetProblem(x.ContestID, x.ProblemID)
			if err != nil {
				return
			}
			req, ok := judge.BuildRequest(x, p)
			if !ok {
				return
			}
			if !pool.Submit(judge.Job{SubID: x.ID, Req: req}) {
				log.Warn("judge queue full, left for manual judging", "sub", x.ID)
			}
		}
		log.Info("autojudge ready", "workers", runtime.NumCPU())
	}

	httpSrv := &http.Server{Addr: cfg.Addr, Handler: srv}
	go func() {
		log.Info("forge starting", "addr", cfg.Addr)
		log.Info("app", "url", "http://localhost"+cfg.Addr+"/app")
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("listen failed", "err", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	stopWorkers()
	ctx2, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(ctx2)
	log.Info("forge stopped")
}
