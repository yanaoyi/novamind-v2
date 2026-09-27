package task

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/yanaoyi/novamindv2/backend/internal/domain"
)

// fakeRepo 是内存版任务仓储，用于确定性地验证 worker 行为。
type fakeRepo struct {
	mu        sync.Mutex
	pending   []domain.Task
	statuses  map[string]domain.TaskStatus
	progress  map[string]int
	messages  map[string]string
	outputs   map[string]map[string]any
	failCalls map[string][]string
	cancelled map[string]bool
	maxAtt    map[string]int
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		statuses:  map[string]domain.TaskStatus{},
		progress:  map[string]int{},
		messages:  map[string]string{},
		outputs:   map[string]map[string]any{},
		failCalls: map[string][]string{},
		cancelled: map[string]bool{},
		maxAtt:    map[string]int{},
	}
}

func (f *fakeRepo) enqueue(t domain.Task) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if t.MaxAttempts == 0 {
		t.MaxAttempts = 3
	}
	t.Status = domain.TaskPending
	f.pending = append(f.pending, t)
	f.statuses[t.ID] = domain.TaskPending
	f.maxAtt[t.ID] = t.MaxAttempts
}

func (f *fakeRepo) ClaimNext(_ context.Context) (*domain.Task, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.pending) == 0 {
		return nil, nil
	}
	t := f.pending[0]
	f.pending = f.pending[1:]
	f.statuses[t.ID] = domain.TaskRunning
	return &t, nil
}

func (f *fakeRepo) UpdateProgress(_ context.Context, id string, progress int, message string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.progress[id] = progress
	f.messages[id] = message
	return nil
}

func (f *fakeRepo) Complete(_ context.Context, id string, output map[string]any) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.statuses[id] = domain.TaskCompleted
	f.outputs[id] = output
	return nil
}

func (f *fakeRepo) Fail(_ context.Context, id string, errMsg string) (domain.TaskStatus, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failCalls[id] = append(f.failCalls[id], errMsg)
	// 与真实仓储一致：没到上限就回到 PENDING，否则 FAILED
	if len(f.failCalls[id]) < f.maxAtt[id] {
		f.statuses[id] = domain.TaskPending
		return domain.TaskPending, nil
	}
	f.statuses[id] = domain.TaskFailed
	return domain.TaskFailed, nil
}

func (f *fakeRepo) IsCancelled(_ context.Context, id string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.cancelled[id], nil
}

func (f *fakeRepo) status(id string) domain.TaskStatus {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.statuses[id]
}

func newTestWorker(repo Repository, reg *Registry) *Worker {
	return NewWorker(repo, reg, nil)
}

func TestWorkerRunsHandlerAndCompletes(t *testing.T) {
	repo := newFakeRepo()
	reg := NewRegistry()
	reg.Register("demo", func(_ context.Context, _ domain.Task, r Reporter) (map[string]any, error) {
		r.Progress(50, "处理中")
		return map[string]any{"done": true}, nil
	})
	repo.enqueue(domain.Task{ID: "t1", Type: "demo"})

	processed, err := newTestWorker(repo, reg).RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce 出错: %v", err)
	}
	if !processed {
		t.Fatal("应处理了一个任务")
	}
	if got := repo.status("t1"); got != domain.TaskCompleted {
		t.Fatalf("任务应完成，实际 %s", got)
	}
	if repo.outputs["t1"]["done"] != true {
		t.Errorf("output 未写入: %v", repo.outputs["t1"])
	}

	// 队列空时应返回 false 而不是错误
	processed, err = newTestWorker(repo, reg).RunOnce(context.Background())
	if err != nil || processed {
		t.Fatalf("空队列应返回 (false, nil)，实际 (%v, %v)", processed, err)
	}
}

func TestWorkerRetriesThenSucceeds(t *testing.T) {
	repo := newFakeRepo()
	reg := NewRegistry()
	var calls int
	reg.Register("flaky", func(_ context.Context, _ domain.Task, r Reporter) (map[string]any, error) {
		calls++
		if calls == 1 {
			return nil, errors.New("第一次失败")
		}
		r.Progress(100, "好了")
		return map[string]any{"calls": calls}, nil
	})

	task := domain.Task{ID: "t2", Type: "flaky", MaxAttempts: 3}
	repo.enqueue(task)
	worker := newTestWorker(repo, reg)

	// 第一次：失败 → 回到待执行
	if _, err := worker.RunOnce(context.Background()); err != nil {
		t.Fatalf("第一次执行出错: %v", err)
	}
	if got := repo.status("t2"); got != domain.TaskPending {
		t.Fatalf("首次失败应回到待执行，实际 %s", got)
	}
	// 重新入队（真实仓储里 Fail 会让它继续被 ClaimNext 领取，这里模拟回来）
	repo.mu.Lock()
	repo.pending = append(repo.pending, task)
	repo.mu.Unlock()

	if _, err := worker.RunOnce(context.Background()); err != nil {
		t.Fatalf("第二次执行出错: %v", err)
	}
	if got := repo.status("t2"); got != domain.TaskCompleted {
		t.Fatalf("重试后应完成，实际 %s", got)
	}
	if len(repo.failCalls["t2"]) != 1 {
		t.Errorf("应只记录 1 次失败，实际 %d 次", len(repo.failCalls["t2"]))
	}
}

func TestWorkerFailsAfterMaxAttempts(t *testing.T) {
	repo := newFakeRepo()
	reg := NewRegistry()
	reg.Register("always_fail", func(_ context.Context, _ domain.Task, _ Reporter) (map[string]any, error) {
		return nil, errors.New("永远失败")
	})
	task := domain.Task{ID: "t3", Type: "always_fail", MaxAttempts: 2}
	worker := newTestWorker(repo, reg)

	repo.enqueue(task)
	_, _ = worker.RunOnce(context.Background())
	if got := repo.status("t3"); got != domain.TaskPending {
		t.Fatalf("第 1 次失败后应待重试，实际 %s", got)
	}

	repo.mu.Lock()
	repo.pending = append(repo.pending, task)
	repo.mu.Unlock()
	_, _ = worker.RunOnce(context.Background())
	if got := repo.status("t3"); got != domain.TaskFailed {
		t.Fatalf("超过重试上限应失败，实际 %s", got)
	}
}

func TestWorkerRecoversFromPanic(t *testing.T) {
	repo := newFakeRepo()
	reg := NewRegistry()
	reg.Register("panic_task", func(_ context.Context, _ domain.Task, _ Reporter) (map[string]any, error) {
		panic("handler 崩了")
	})
	task := domain.Task{ID: "t4", Type: "panic_task", MaxAttempts: 1}
	repo.enqueue(task)

	if _, err := newTestWorker(repo, reg).RunOnce(context.Background()); err != nil {
		t.Fatalf("worker 不应因 handler panic 而报错: %v", err)
	}
	if got := repo.status("t4"); got != domain.TaskFailed {
		t.Fatalf("panic 应记为失败，实际 %s", got)
	}
	if len(repo.failCalls["t4"]) != 1 || !strings.Contains(repo.failCalls["t4"][0], "panic") {
		t.Errorf("失败原因应包含 panic 信息，实际 %v", repo.failCalls["t4"])
	}
}

func TestWorkerRejectsUnknownType(t *testing.T) {
	repo := newFakeRepo()
	reg := NewRegistry() // 不注册任何 handler
	repo.enqueue(domain.Task{ID: "t5", Type: "no_such_type", MaxAttempts: 1})

	if _, err := newTestWorker(repo, reg).RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce 出错: %v", err)
	}
	if got := repo.status("t5"); got != domain.TaskFailed {
		t.Fatalf("未知任务类型应失败，实际 %s", got)
	}
	if !strings.Contains(repo.failCalls["t5"][0], "未知任务类型") {
		t.Errorf("失败原因应说明类型未知，实际 %v", repo.failCalls["t5"])
	}
}

func TestWorkerDoesNotOverwriteCancelledTask(t *testing.T) {
	repo := newFakeRepo()
	reg := NewRegistry()
	reg.Register("cancellable", func(_ context.Context, t domain.Task, r Reporter) (map[string]any, error) {
		// 模拟外部在任务执行途中取消
		repo.mu.Lock()
		repo.cancelled[t.ID] = true
		repo.statuses[t.ID] = domain.TaskCancelled
		repo.mu.Unlock()
		r.Progress(50, "进行中")
		return map[string]any{"ok": true}, nil
	})
	repo.enqueue(domain.Task{ID: "t6", Type: "cancellable"})

	if _, err := newTestWorker(repo, reg).RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce 出错: %v", err)
	}
	if got := repo.status("t6"); got != domain.TaskCancelled {
		t.Fatalf("已取消的任务不应被改写成完成，实际 %s", got)
	}
	if _, ok := repo.outputs["t6"]; ok {
		t.Error("已取消的任务不应写入 output")
	}
}

func TestRegistryTypes(t *testing.T) {
	reg := NewRegistry()
	reg.Register("b_task", func(context.Context, domain.Task, Reporter) (map[string]any, error) { return nil, nil })
	reg.Register("a_task", func(context.Context, domain.Task, Reporter) (map[string]any, error) { return nil, nil })

	types := reg.Types()
	if len(types) != 2 || types[0] != "a_task" || types[1] != "b_task" {
		t.Fatalf("类型应排序返回，实际 %v", types)
	}
	if _, ok := reg.Lookup("a_task"); !ok {
		t.Error("应能查到已注册类型")
	}
	if _, ok := reg.Lookup("nope"); ok {
		t.Error("未注册类型不应查到")
	}
}

func TestInputString(t *testing.T) {
	input := map[string]any{"work_id": " abc ", "num": 12}
	if got := InputString(input, "work_id"); got != "abc" {
		t.Errorf("应去掉首尾空白，实际 %q", got)
	}
	if got := InputString(input, "num"); got != "" {
		t.Errorf("非字符串应返回空串，实际 %q", got)
	}
	if got := InputString(input, "missing"); got != "" {
		t.Errorf("缺失键应返回空串，实际 %q", got)
	}
	if got := InputString(nil, "work_id"); got != "" {
		t.Errorf("nil 入参应返回空串，实际 %q", got)
	}
}
