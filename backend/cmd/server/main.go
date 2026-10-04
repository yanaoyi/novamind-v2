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

	"github.com/yanaoyi/novamindv2/backend/internal/ai"
	"github.com/yanaoyi/novamindv2/backend/internal/api"
	"github.com/yanaoyi/novamindv2/backend/internal/config"
	"github.com/yanaoyi/novamindv2/backend/internal/infra"
	"github.com/yanaoyi/novamindv2/backend/internal/repository"
	"github.com/yanaoyi/novamindv2/backend/internal/service"
	"github.com/yanaoyi/novamindv2/backend/internal/storage"
	"github.com/yanaoyi/novamindv2/backend/internal/task"
)

// version 由构建时注入：go build -ldflags "-X main.version=..."
var version = "0.1.0-dev"

// @title			NovaMind API
// @version		0.1.0
// @description	NovaMind V2 后端 API：原著分析 → 二创设计 → 写作 → 一致性检查。所有响应统一为 {data, error, trace_id}。
// @BasePath		/api/v1
// @schemes		http
// @accept			json
// @produce		json
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

	// PostgreSQL：配置了就必须可用，没配置直接拒绝启动。
	// 说明：早期版本在缺 DATABASE_URL 时"降级启动"，结果工程路由不会注册、
	// 接口静默返回 404 —— 这种"残废服务"比启动失败更难排查，故改为 fail fast。
	if cfg.DatabaseURL == "" {
		return errors.New("DATABASE_URL 未配置：请在 backend/.env 中配置，" +
			"或在 backend 目录下启动（.env 按当前工作目录查找）")
	}
	pg, err := infra.NewPostgres(context.Background(), cfg.DatabaseURL, cfg.LogLevel == "debug")
	if err != nil {
		return err
	}
	defer func() { _ = pg.Close() }()
	deps.Postgres = pg.Health
	logger.Info("PostgreSQL 已连接")

	// 文件存储（Phase 2 用本地文件系统）
	fileStore, err := storage.NewLocalStore(cfg.StorageDir)
	if err != nil {
		return err
	}
	logger.Info("文件存储就绪", slog.String("root", fileStore.Root()))

	projectRepo := repository.NewProjectRepo(pg.DB)
	projects := service.NewProjectService(projectRepo)
	originals := service.NewOriginalService(
		repository.NewOriginalRepo(pg.DB),
		projectRepo,
		fileStore,
		cfg.UploadMaxBytes(),
	)
	originalRepo := repository.NewOriginalRepo(pg.DB)
	characters := service.NewOriginalCharacterService(
		repository.NewOriginalCharacterRepo(pg.DB),
		originalRepo,
	)
	worlds := service.NewOriginalWorldService(
		repository.NewOriginalWorldRepo(pg.DB),
		originalRepo,
	)
	characterRepo := repository.NewOriginalCharacterRepo(pg.DB)
	worldRepo := repository.NewOriginalWorldRepo(pg.DB)
	events := service.NewOriginalEventService(
		repository.NewOriginalEventRepo(pg.DB),
		originalRepo,
		characterRepo,
		worldRepo,
	)

	// AI 接入（规格书 §36 Model Gateway / §37 Prompt Engine）
	gateway := ai.NewGateway()
	providers := service.NewModelProviderService(
		repository.NewModelProviderRepo(pg.DB),
		gateway,
		cfg.SecretKey,
	)
	if cfg.SecretKey == "" {
		logger.Warn("NOVAMIND_SECRET 未配置：暂时无法保存模型 API Key（见 .env.example）")
	}
	promptEngine, err := ai.NewEngine()
	if err != nil {
		return err
	}
	logger.Info("Prompt 模板已加载", slog.Int("count", len(promptEngine.List())))

	// 异步任务：注册处理函数并启动 worker（规格书 §53）
	taskRepo := repository.NewTaskRepo(pg.DB)
	registry := task.NewRegistry()
	task.RegisterOriginalHandlers(registry, originals)
	tasks := service.NewTaskService(taskRepo, originalRepo, func(taskType string) bool {
		_, ok := registry.Lookup(taskType)
		return ok
	})
	worker := task.NewWorker(taskRepo, registry, logger,
		task.WithConcurrency(2), task.WithPollInterval(2*time.Second))

	// AI 分析：模型调用器 + 提案服务（AI 只产提案，作者通过后才写原著模型）
	invoker := service.NewModelInvoker(providers, promptEngine, gateway)
	analysis := service.NewAnalysisService(
		repository.NewAnalysisProposalRepo(pg.DB),
		originalRepo,
		repository.NewOriginalRepo(pg.DB),
		tasks,
	)
	task.RegisterAnalysisHandlers(registry, analysis, invoker)

	// 二创（规格书 §17-§23）：人物继承、融合、映射
	creative := service.NewCreativeService(
		repository.NewCreativeRepo(pg.DB),
		projectRepo,
		originalRepo,
		characterRepo,
		repository.NewOriginalWorldRepo(pg.DB),
		repository.NewOriginalEventRepo(pg.DB),
	)
	// 写作服务：仓储 + 二创作品查询（repo 直接提供 GetWorkByID）+ 上下文读取（CreativeService 提供人物/世界）
	writing := service.NewWritingService(
		repository.NewWritingRepo(pg.DB),
		repository.NewCreativeRepo(pg.DB),
		creative,
	)
	task.RegisterWritingHandlers(registry, writing, invoker)
	// 大纲（规格书 §27）：卷 → 节 → 章的独立模型 + 落成章节
	outlines := service.NewOutlineService(
		repository.NewOutlineRepo(pg.DB),
		repository.NewCreativeRepo(pg.DB),
		repository.NewWritingRepo(pg.DB),
		repository.NewEntityVersionRepo(pg.DB),
	)
	// 版本历史：人物 / 世界观 / 大纲的状态快照（规格书 §59）
	versions := service.NewVersionService(repository.NewEntityVersionRepo(pg.DB), creative, writing)
	versions.SetOutlineTreeVersioner(outlines)

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

	srv := api.NewServer(
		cfg, logger, deps,
		projects, originals, characters, worlds, events,
		providers, promptEngine,
		tasks, registry.Types,
		analysis,
		creative,
		writing,
		outlines,
		versions,
		invoker,
	)

	workerCtx, stopWorker := context.WithCancel(context.Background())
	defer stopWorker()
	go worker.Run(workerCtx)

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
