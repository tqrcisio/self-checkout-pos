package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/tqrcisio/self-checkout-pos/internal/config"
	"github.com/tqrcisio/self-checkout-pos/internal/db"
	"github.com/tqrcisio/self-checkout-pos/internal/server"
	"github.com/tqrcisio/self-checkout-pos/internal/updater"
)

type App struct {
	ctx       context.Context
	version   string
	logger    *slog.Logger
	db        *sql.DB
	srv       *server.Server
	srvCtx    context.Context
	srvCancel context.CancelFunc
	srvWG     sync.WaitGroup
	updater   *updater.Updater
}

func NewApp(version string) *App {
	return &App{
		version: version,
		logger:  slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})),
	}
}

func (a *App) OnStartup(ctx context.Context) {
	a.ctx = ctx
	a.logger.Info("app starting", "version", a.version)

	updater.SetVersion(a.version)

	database, err := db.Open()
	if err != nil {
		a.logger.Error("open db", "err", err)
		return
	}
	a.db = database

	if err := db.Migrate(a.db); err != nil {
		a.logger.Error("migrate db", "err", err)
		return
	}

	cfg, err := config.Load()
	if err != nil {
		a.logger.Warn("config load failed, using defaults", "err", err)
		cfg = config.Default()
	}

	a.srv = server.New(cfg)

	if exeDir, err := config.ExecutableDir(); err == nil && updater.Version() != "dev" {
		a.updater = updater.Start(exeDir, a.srv.Config)
		a.srv.SetUpdater(a.updater)
	}

	a.srvCtx, a.srvCancel = context.WithCancel(ctx)
	a.srvWG.Add(1)
	go func() {
		defer a.srvWG.Done()
		if err := a.srv.Start(); err != nil && err != http.ErrServerClosed {
			a.logger.Error("http server", "err", err)
		}
	}()
}

func (a *App) OnDomReady(ctx context.Context) {}

func (a *App) OnShutdown(ctx context.Context) {
	a.logger.Info("app shutting down")

	if a.srv != nil {
		a.srv.Stop()
	}
	if a.srvCancel != nil {
		a.srvCancel()
	}

	done := make(chan struct{})
	go func() {
		a.srvWG.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		a.logger.Warn("http server did not stop within 5s")
	}

	if a.updater != nil {
		a.updater.Stop()
	}

	if a.db != nil {
		if err := a.db.Close(); err != nil {
			a.logger.Error("close db", "err", err)
		}
	}
}

func (a *App) Hello(name string) string {
	if name == "" {
		name = "POS"
	}
	return fmt.Sprintf("Hello, %s! Backend version: %s", name, a.version)
}
