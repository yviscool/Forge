package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yviscool/forge/internal/adapters/store/memory"
	"github.com/yviscool/forge/internal/adapters/store/sqlite"
	"github.com/yviscool/forge/internal/app"
	"github.com/yviscool/forge/internal/config"
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
	handler := httpapi.NewServer(svc, log)

	srv := &http.Server{Addr: cfg.Addr, Handler: handler}
	go func() {
		log.Info("forge starting", "addr", cfg.Addr)
		log.Info("app", "url", "http://localhost"+cfg.Addr+"/app")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("listen failed", "err", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	log.Info("forge stopped")
}
