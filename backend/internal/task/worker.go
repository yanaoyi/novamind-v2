// Package task 是异步任务执行框架（SPEC.md §27、§22.1）。
//
// 设计要点：
//   - 队列落在 PostgreSQL（领取用 FOR UPDATE SKIP LOCKED）：任务状态与业务数据同库，
//     重启不丢任务，部署不需要额外中间件；将来要换 Asynq 只需替换 Worker 的取任务方式。
//   - 任务类型 → 处理函数用注册表挂载，新增任务类型不影响框架。
//   - handler 通过 Reporter 上报进度并感知取消；panic 会被兜住并按失败处理。
package task

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/yanaoyi/novamindv2/backend/internal/domain"
)

// Repository 是 worker 需要的仓储能力（接口化便于单测替身）。
type Repository interface {
	ClaimNext(ctx context.Context) (*domain.Task, error)
	UpdateProgress(ctx context.Context, id string, progress int, message string) error
	Complete(ctx context.Context, id string, output map[string]any) error
	Fail(ctx context.Context, id string, errMsg string) (domain.TaskStatus, error)
	IsCancelled(ctx context.Context, id string) (bool, error)
}

// Reporter 供 handler 上报进度与检查取消。
type Reporter interface {
	// Progress 汇报进度（percent 0-100）。实现内部会节流，可放心频繁调用。
	Progress(percent int, message string)
	// Cancelled 返回任务是否已被取消；长任务应在关键阶段之间检查。
	Cancelled() bool
}

// Handler 是任务处理函数。
// 返回的 output 会写入 tasks.output；返回 error 时按重试策略处理。
type Handler func(ctx context.Context, t domain.Task, r Reporter) (map[string]any, error)

// Registry 是任务类型注册表。
type Registry struct {
	mu       sync.RWMutex
	handlers map[string]Handler
}

// NewRegistry 构建注册表。
func NewRegistry() *Registry {
	return &Registry{handlers: map[string]Handler{}}
}

// Register 注册任务处理函数。
func (r *Registry) Register(taskType string, h Handler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[taskType] = h
}

// Lookup 查找处理函数。
func (r *Registry) Lookup(taskType string) (Handler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	h, ok := r.handlers[taskType]
	return h, ok
}

// Types 返回已注册的任务类型（便于界面展示与自检）。
func (r *Registry) Types() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.handlers))
	for k := range r.handlers {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Worker 是任务执行器。
type Worker struct {
	repo        Repository
	registry    *Registry
	logger      *slog.Logger
	concurrency int
	pollEvery   time.Duration
}

// Option 是 Worker 可选项。
type Option func(*Worker)

// WithConcurrency 设置并发执行数。
func WithConcurrency(n int) Option {
	return func(w *Worker) {
		if n > 0 {
			w.concurrency = n
		}
	}
}

// WithPollInterval 设置空队列时的轮询间隔。
func WithPollInterval(d time.Duration) Option {
	return func(w *Worker) {
		if d > 0 {
			w.pollEvery = d
		}
	}
}

// NewWorker 构建执行器。
func NewWorker(repo Repository, registry *Registry, logger *slog.Logger, opts ...Option) *Worker {
	w := &Worker{
		repo: repo, registry: registry, logger: logger,
		concurrency: 2, pollEvery: 2 * time.Second,
	}
	for _, opt := range opts {
		opt(w)
	}
	return w
}

// Run 启动 worker 直到 ctx 结束。
func (w *Worker) Run(ctx context.Context) {
	var wg sync.WaitGroup
	for i := 0; i < w.concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			w.loop(ctx, id)
		}(i)
	}
	if w.logger != nil {
		w.logger.Info("任务 worker 已启动",
			slog.Int("concurrency", w.concurrency),
			slog.Any("types", w.registry.Types()),
		)
	}
	wg.Wait()
}

func (w *Worker) loop(ctx context.Context, workerID int) {
	for {
		if ctx.Err() != nil {
			return
		}
		processed, err := w.RunOnce(ctx)
		if err != nil && w.logger != nil {
			w.logger.Error("任务处理异常", slog.Int("worker", workerID), slog.Any("error", err))
		}
		if !processed {
			select {
			case <-ctx.Done():
				return
			case <-time.After(w.pollEvery):
			}
		}
	}
}

// RunOnce 领取并执行一个任务；返回是否真的处理了任务。
// 测试可以直接调用它做确定性验证。
func (w *Worker) RunOnce(ctx context.Context) (bool, error) {
	t, err := w.repo.ClaimNext(ctx)
	if err != nil {
		return false, err
	}
	if t == nil {
		return false, nil
	}
	w.execute(ctx, *t)
	return true, nil
}

func (w *Worker) execute(ctx context.Context, t domain.Task) {
	handler, ok := w.registry.Lookup(t.Type)
	if !ok {
		status, err := w.repo.Fail(ctx, t.ID, fmt.Sprintf("未知任务类型：%s", t.Type))
		w.logTaskEnd(t, status, err)
		return
	}

	reporter := newDBReporter(ctx, w.repo, t.ID)
	reporter.Progress(5, "开始执行")

	output, err := safeCall(ctx, handler, t, reporter)

	// 已被取消：不再写完成/失败状态（Cancel 已经把状态置为 CANCELLED）
	if cancelled, checkErr := w.repo.IsCancelled(context.WithoutCancel(ctx), t.ID); checkErr == nil && cancelled {
		if w.logger != nil {
			w.logger.Info("任务已取消", slog.String("task_id", t.ID), slog.String("type", t.Type))
		}
		return
	}

	if err == nil {
		if completeErr := w.repo.Complete(context.WithoutCancel(ctx), t.ID, output); completeErr != nil {
			w.logTaskEnd(t, domain.TaskFailed, completeErr)
		}
		return
	}

	status, failErr := w.repo.Fail(context.WithoutCancel(ctx), t.ID, err.Error())
	if failErr != nil {
		w.logTaskEnd(t, domain.TaskFailed, failErr)
		return
	}
	w.logTaskEnd(t, status, nil)
}

func (w *Worker) logTaskEnd(t domain.Task, status domain.TaskStatus, err error) {
	if w.logger == nil {
		return
	}
	attrs := []any{slog.String("task_id", t.ID), slog.String("type", t.Type), slog.String("status", string(status))}
	if err != nil {
		attrs = append(attrs, slog.Any("error", err))
	}
	w.logger.Info("任务执行结束", attrs...)
}

// safeCall 调用 handler 并兜住 panic —— 一个任务把 worker 打挂是最难查的故障。
func safeCall(ctx context.Context, h Handler, t domain.Task, r Reporter) (output map[string]any, err error) {
	defer func() {
		if rec := recover(); rec != nil {
			err = fmt.Errorf("任务 panic: %v", rec)
		}
	}()
	return h(ctx, t, r)
}

// dbReporter 把进度写回数据库，并带节流与取消检查。
type dbReporter struct {
	ctx      context.Context
	repo     Repository
	taskID   string
	mu       sync.Mutex
	lastAt   time.Time
	cancelCh atomic.Bool
}

func newDBReporter(ctx context.Context, repo Repository, taskID string) *dbReporter {
	return &dbReporter{ctx: ctx, repo: repo, taskID: taskID}
}

// Progress 写进度：距上次写入不足 800ms 时跳过（percent=100 强制写）。
func (r *dbReporter) Progress(percent int, message string) {
	if percent > 100 {
		percent = 100
	}
	r.mu.Lock()
	now := time.Now()
	if percent < 100 && now.Sub(r.lastAt) < 800*time.Millisecond {
		r.mu.Unlock()
		return
	}
	r.lastAt = now
	r.mu.Unlock()

	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(r.ctx), 5*time.Second)
	defer cancel()
	if err := r.repo.UpdateProgress(writeCtx, r.taskID, percent, truncate(message, 200)); err != nil {
		// 进度写失败不该影响任务本身，只记录到取消标记不置位
		return
	}
	// 顺带刷新取消标记（每次写进度查一次状态，成本很低）
	if cancelled, err := r.repo.IsCancelled(writeCtx, r.taskID); err == nil && cancelled {
		r.cancelCh.Store(true)
	}
}

// Cancelled 返回是否已被取消。
func (r *dbReporter) Cancelled() bool { return r.cancelCh.Load() }

func truncate(s string, max int) string {
	runes := []rune(strings.TrimSpace(s))
	if len(runes) <= max {
		return string(runes)
	}
	return string(runes[:max])
}

// InputString 从任务入参里取字符串（handler 常用）。
func InputString(input map[string]any, key string) string {
	if input == nil {
		return ""
	}
	v, ok := input[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(s)
}

// ErrUnknownTaskType 供外部在注册缺失时统一报错文案。
var ErrUnknownTaskType = errors.New("未知任务类型")
