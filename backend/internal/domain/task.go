package domain

import (
	"errors"
	"strings"
	"time"
)

// 任务相关错误。
var (
	ErrTaskTypeEmpty      = errors.New("任务类型不能为空")
	ErrTaskNotFound       = errors.New("任务不存在")
	ErrTaskNotCancellable = errors.New("该任务当前状态不能取消")
	ErrTaskNotRetryable   = errors.New("该任务当前状态不能重试")
	ErrTaskStatusInvalid  = errors.New("任务状态非法")
)

// TaskStatus 是任务状态（SPEC.md §27）。
type TaskStatus string

const (
	TaskPending   TaskStatus = "PENDING"
	TaskRunning   TaskStatus = "RUNNING"
	TaskPaused    TaskStatus = "PAUSED"
	TaskCompleted TaskStatus = "COMPLETED"
	TaskFailed    TaskStatus = "FAILED"
	TaskCancelled TaskStatus = "CANCELLED"
)

// Valid 判断状态是否合法。
func (s TaskStatus) Valid() bool {
	switch s {
	case TaskPending, TaskRunning, TaskPaused, TaskCompleted, TaskFailed, TaskCancelled:
		return true
	default:
		return false
	}
}

// IsTerminal 判断是否终态。
func (s TaskStatus) IsTerminal() bool {
	return s == TaskCompleted || s == TaskCancelled || s == TaskFailed
}

// Task 是一个异步任务。
type Task struct {
	ID              string
	ProjectID       *string
	WorkID          *string
	CreativeWorkID  *string
	Type            string
	Status          TaskStatus
	Progress        int
	ProgressMessage string
	Input           map[string]any
	Output          map[string]any
	Error           string
	Attempts        int
	MaxAttempts     int
	CreatedAt       time.Time
	StartedAt       *time.Time
	FinishedAt      *time.Time
	UpdatedAt       time.Time
	DeletedAt       *time.Time
}

// Validate 校验任务。
func (t *Task) Validate() error {
	if strings.TrimSpace(t.Type) == "" {
		return ErrTaskTypeEmpty
	}
	if t.Status != "" && !t.Status.Valid() {
		return ErrTaskStatusInvalid
	}
	return nil
}

// Normalize 补默认值。
func (t *Task) Normalize() {
	t.Type = strings.TrimSpace(t.Type)
	if t.Status == "" {
		t.Status = TaskPending
	}
	if t.MaxAttempts <= 0 {
		t.MaxAttempts = 3
	}
	if t.Input == nil {
		t.Input = map[string]any{}
	}
	if t.Output == nil {
		t.Output = map[string]any{}
	}
}

// CanCancel 判断任务能否取消。
// 运行中的任务也允许取消（worker 会在下一次进度回调时看到取消标记并退出）。
func (t *Task) CanCancel() bool {
	return t.Status == TaskPending || t.Status == TaskRunning || t.Status == TaskPaused
}

// CanRetry 判断任务能否重试（失败或被取消的任务可以重来）。
func (t *Task) CanRetry() bool {
	return t.Status == TaskFailed || t.Status == TaskCancelled
}
