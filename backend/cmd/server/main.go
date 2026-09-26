// NovaMind V2 后端入口。
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yanaoyi/novamindv2/backend/internal/api"
	"github.com/yanaoyi/novamindv2/backend/internal/config"
	"github.com/yanaoyi/novamindv2/backend/internal/infra"
)

// version 由构建时注入：go build -ldflags "-X main.version=..."
var version = "0.1.0-dev"

func main() {
	if err := run(); err != nil {
		slog.Error("服务退出", slog.Any("error", err))
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	logger := newLogger(cfg)

	deps := api.HealthDeps{
		Version: version,
		Env:     cfg.AppEnv,
		Started: time.Now(),
	}

	// PostgreSQL：连接失败即启动失败（配置了就必须可用，避免"假装健康"）
	if cfg.DatabaseURL == "" {
		logger.Warn("DATABASE_URL 未配置，跳过 PostgreSQL 连接")
	} else {
		pg, err := infra.NewPostgres(context.Background(), cfg.DatabaseURL, cfg.LogLevel == "debug")
		if err != nil {
			return err
		}
		defer func() { _ = pg.Close() }()
		deps.Postgres = pg.Health
		logger.Info("PostgreSQL 已连接")
	}

	// Redis
	if cfg.RedisAddr == "" {
		logger.Warn("REDIS_ADDR 未配置，跳过 Redis 连接")
	} else {
		rdb, err := infra.NewRedis(context.Background(), cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
		if err != nil {
			return err
		}
		defer func() { _ = rdb.Close() }()
		deps.Redis = rdb.Health
		logger.Info("Redis 已连接")
	}

	srv := api.NewServer(cfg, logger, deps)

	httpServer := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           srv.Router(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("服务启动", slog.String("addr", cfg.HTTPAddr), slog.String("env", cfg.AppEnv), slog.String("version", version))
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return err
	case sig := <-stop:
		logger.Info("收到退出信号，开始优雅关闭", slog.String("signal", sig.String()))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return httpServer.Shutdown(ctx)
}

func newLogger(cfg *config.Config) *slog.Logger {
	level := slog.LevelInfo
	switch cfg.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	var handler slog.Handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	if cfg.IsProduction() {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	}
	logger := slog.New(handler)
	slog.SetDefault(logger)
	return logger
}
