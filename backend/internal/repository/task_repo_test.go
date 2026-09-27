package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/yanaoyi/novamindv2/backend/internal/domain"
)

// newTaskRepoForTest 复用 project_repo_test 里的新库探测逻辑（每个测试独立事务，结束回滚）。
func newTaskRepoForTest(t *testing.T) *TaskRepo {
	t.Helper()
	repo := newTestRepo(t) // 复用：内部已保证"有库 + 事务回滚"
	return NewTaskRepo(repo.db)
}

func TestTaskRepoClaimAndComplete(t *testing.T) {
	ctx := context.Background()
	repo := newTaskRepoForTest(t)

	task := &domain.Task{Type: "unit_test_claim", Input: map[string]any{"k": "v"}}
	task.Normalize()
	if err := repo.Create(ctx, task); err != nil {
		t.Fatalf("创建任务失败: %v", err)
	}
	if task.Status != domain.TaskPending {
		t.Fatalf("新任务应为 PENDING，实际 %s", task.Status)
	}

	claimed, err := repo.ClaimNext(ctx)
	if err != nil {
		t.Fatalf("领取任务失败: %v", err)
	}
	if claimed == nil {
		t.Fatal("应领取到任务")
	}
	if claimed.ID != task.ID || claimed.Status != domain.TaskRunning {
		t.Fatalf("领取结果不正确: %+v", claimed)
	}
	if claimed.Attempts != 1 {
		t.Errorf("领取后 attempts 应为 1，实际 %d", claimed.Attempts)
	}
	if claimed.Input["k"] != "v" {
		t.Errorf("入参未正确往返: %v", claimed.Input)
	}

	if err := repo.UpdateProgress(ctx, task.ID, 60, "一半了"); err != nil {
		t.Fatalf("更新进度失败: %v", err)
	}
	if err := repo.Complete(ctx, task.ID, map[string]any{"ok": true}); err != nil {
		t.Fatalf("完成任务失败: %v", err)
	}

	got, err := repo.GetByID(ctx, task.ID)
	if err != nil {
		t.Fatalf("查询任务失败: %v", err)
	}
	if got.Status != domain.TaskCompleted || got.Progress != 100 {
		t.Fatalf("完成状态不正确: %s / %d", got.Status, got.Progress)
	}
	if got.Output["ok"] != true {
		t.Errorf("输出未写入: %v", got.Output)
	}
	if got.FinishedAt == nil {
		t.Error("完成时间未写入")
	}
}

func TestTaskRepoQueueIsEmptyWhenAllClaimed(t *testing.T) {
	ctx := context.Background()
	repo := newTaskRepoForTest(t)

	// 事务隔离下这里只有本测试创建的任务
	task := &domain.Task{Type: "unit_test_empty"}
	task.Normalize()
	if err := repo.Create(ctx, task); err != nil {
		t.Fatalf("创建任务失败: %v", err)
	}
	if _, err := repo.ClaimNext(ctx); err != nil {
		t.Fatalf("首次领取失败: %v", err)
	}
	again, err := repo.ClaimNext(ctx)
	if err != nil {
		t.Fatalf("再次领取失败: %v", err)
	}
	if again != nil {
		t.Fatalf("队列应已空，却领到任务 %s", again.ID)
	}
}

func TestTaskRepoFailRetriesThenFails(t *testing.T) {
	ctx := context.Background()
	repo := newTaskRepoForTest(t)

	task := &domain.Task{Type: "unit_test_retry", MaxAttempts: 2}
	task.Normalize()
	if err := repo.Create(ctx, task); err != nil {
		t.Fatalf("创建任务失败: %v", err)
	}

	// 第 1 次：领取 → 失败 → 回到 PENDING（还有重试机会）
	if _, err := repo.ClaimNext(ctx); err != nil {
		t.Fatalf("领取失败: %v", err)
	}
	status, err := repo.Fail(ctx, task.ID, "第一次失败")
	if err != nil {
		t.Fatalf("记录失败出错: %v", err)
	}
	if status != domain.TaskPending {
		t.Fatalf("未到上限应回到 PENDING，实际 %s", status)
	}
	afterFirst, _ := repo.GetByID(ctx, task.ID)
	if afterFirst.Status != domain.TaskPending || afterFirst.Error == "" {
		t.Fatalf("重试排队状态不正确: %+v", afterFirst)
	}

	// 第 2 次：领取 → 再失败 → FAILED（已达上限）
	if _, err := repo.ClaimNext(ctx); err != nil {
		t.Fatalf("第二次领取失败: %v", err)
	}
	status, err = repo.Fail(ctx, task.ID, "第二次失败")
	if err != nil {
		t.Fatalf("记录失败出错: %v", err)
	}
	if status != domain.TaskFailed {
		t.Fatalf("达到上限应 FAILED，实际 %s", status)
	}

	// 重试：重置为 PENDING 且 attempts 归零
	if err := repo.Retry(ctx, task.ID); err != nil {
		t.Fatalf("重试失败: %v", err)
	}
	afterRetry, _ := repo.GetByID(ctx, task.ID)
	if afterRetry.Status != domain.TaskPending || afterRetry.Attempts != 0 || afterRetry.Error != "" {
		t.Fatalf("重试后状态不正确: %+v", afterRetry)
	}
}

func TestTaskRepoCancelSemantics(t *testing.T) {
	ctx := context.Background()
	repo := newTaskRepoForTest(t)

	task := &domain.Task{Type: "unit_test_cancel"}
	task.Normalize()
	if err := repo.Create(ctx, task); err != nil {
		t.Fatalf("创建任务失败: %v", err)
	}

	if err := repo.Cancel(ctx, task.ID); err != nil {
		t.Fatalf("取消待执行任务应成功: %v", err)
	}
	cancelled, _ := repo.IsCancelled(ctx, task.ID)
	if !cancelled {
		t.Fatal("IsCancelled 应为 true")
	}

	// 已取消的任务不能再取消
	if err := repo.Cancel(ctx, task.ID); !errors.Is(err, domain.ErrTaskNotCancellable) {
		t.Fatalf("重复取消应返回 ErrTaskNotCancellable，实际 %v", err)
	}
	// 已取消的任务不能重试（只有 FAILED/CANCELLED 可以；这里验证 CANCELLED 可以重试）
	if err := repo.Retry(ctx, task.ID); err != nil {
		t.Fatalf("取消后的任务应可重试: %v", err)
	}

	// 完成后既不能取消也不能重试
	if _, err := repo.ClaimNext(ctx); err != nil {
		t.Fatalf("领取失败: %v", err)
	}
	if err := repo.Complete(ctx, task.ID, nil); err != nil {
		t.Fatalf("完成任务失败: %v", err)
	}
	if err := repo.Cancel(ctx, task.ID); !errors.Is(err, domain.ErrTaskNotCancellable) {
		t.Fatalf("已完成任务取消应报错，实际 %v", err)
	}
	if err := repo.Retry(ctx, task.ID); !errors.Is(err, domain.ErrTaskNotRetryable) {
		t.Fatalf("已完成任务重试应报错，实际 %v", err)
	}
}

func TestTaskRepoNotFound(t *testing.T) {
	ctx := context.Background()
	repo := newTaskRepoForTest(t)

	if _, err := repo.GetByID(ctx, "00000000-0000-7000-8000-000000000000"); !errors.Is(err, domain.ErrTaskNotFound) {
		t.Fatalf("应返回 ErrTaskNotFound，实际 %v", err)
	}
	if _, err := repo.GetByID(ctx, "not-a-uuid"); !errors.Is(err, domain.ErrTaskNotFound) {
		t.Fatalf("非法 UUID 应返回 ErrTaskNotFound，实际 %v", err)
	}
	if err := repo.Cancel(ctx, "00000000-0000-7000-8000-000000000000"); !errors.Is(err, domain.ErrTaskNotFound) {
		t.Fatalf("取消不存在的任务应返回 ErrTaskNotFound，实际 %v", err)
	}
}

func TestTaskRepoListFilter(t *testing.T) {
	ctx := context.Background()
	repo := newTaskRepoForTest(t)

	for i := 0; i < 3; i++ {
		task := &domain.Task{Type: "unit_test_list"}
		task.Normalize()
		if err := repo.Create(ctx, task); err != nil {
			t.Fatalf("创建任务失败: %v", err)
		}
	}

	items, total, err := repo.List(ctx, TaskFilter{Type: "unit_test_list", Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("列表查询失败: %v", err)
	}
	if total != 3 || len(items) != 3 {
		t.Fatalf("应查到 3 条，实际 total=%d len=%d", total, len(items))
	}
	if items[0].CreatedAt.Before(items[len(items)-1].CreatedAt) {
		t.Error("列表应按创建时间倒序")
	}
}
