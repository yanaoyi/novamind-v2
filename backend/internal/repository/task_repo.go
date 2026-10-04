package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/yanaoyi/novamindv2/backend/internal/domain"
)

type taskModel struct {
	ID              string         `gorm:"column:id;type:uuid;primaryKey"`
	ProjectID       *string        `gorm:"column:project_id;type:uuid"`
	WorkID          *string        `gorm:"column:work_id;type:uuid"`
	CreativeWorkID  *string        `gorm:"column:creative_work_id;type:uuid"`
	Type            string         `gorm:"column:type;size:60;not null"`
	Status          string         `gorm:"column:status;size:20;not null;default:PENDING"`
	Progress        int            `gorm:"column:progress;not null;default:0"`
	ProgressMessage string         `gorm:"column:progress_message;size:200;not null;default:''"`
	Input           string         `gorm:"column:input;type:jsonb;not null;default:'{}'"`
	Output          string         `gorm:"column:output;type:jsonb;not null;default:'{}'"`
	Error           string         `gorm:"column:error;not null;default:''"`
	Attempts        int            `gorm:"column:attempts;not null;default:0"`
	MaxAttempts     int            `gorm:"column:max_attempts;not null;default:3"`
	CreatedAt       time.Time      `gorm:"column:created_at;not null"`
	StartedAt       *time.Time     `gorm:"column:started_at"`
	FinishedAt      *time.Time     `gorm:"column:finished_at"`
	NextRunAt       *time.Time     `gorm:"column:next_run_at"`
	UpdatedAt       time.Time      `gorm:"column:updated_at;not null"`
	DeletedAt       gorm.DeletedAt `gorm:"column:deleted_at;index"`
}

func (taskModel) TableName() string { return "tasks" }

// TaskFilter 是任务列表查询条件。
type TaskFilter struct {
	ProjectID      string
	WorkID         string
	CreativeWorkID string
	Status         string
	Type           string
	Page           int
	PageSize       int
}

// TaskRepo 是任务仓储。
type TaskRepo struct {
	db *gorm.DB
}

// NewTaskRepo 构建仓储。
func NewTaskRepo(db *gorm.DB) *TaskRepo { return &TaskRepo{db: db} }

// Create 创建任务（入队）。
func (r *TaskRepo) Create(ctx context.Context, t *domain.Task) error {
	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("生成任务 ID 失败: %w", err)
	}
	t.ID = id.String()
	now := time.Now().UTC()
	t.CreatedAt, t.UpdatedAt = now, now

	m, err := toTaskModel(t)
	if err != nil {
		return err
	}
	if err := r.db.WithContext(ctx).Create(&m).Error; err != nil {
		if isForeignKeyViolation(err) {
			return errors.New("任务关联的工程或原著不存在")
		}
		return fmt.Errorf("创建任务失败: %w", err)
	}
	return nil
}

// GetByID 取任务。
func (r *TaskRepo) GetByID(ctx context.Context, id string) (*domain.Task, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, domain.ErrTaskNotFound
	}
	var m taskModel
	if err := r.db.WithContext(ctx).First(&m, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrTaskNotFound
		}
		return nil, fmt.Errorf("查询任务失败: %w", err)
	}
	t, err := toDomainTask(m)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// List 分页列出任务。
func (r *TaskRepo) List(ctx context.Context, f TaskFilter) ([]domain.Task, int64, error) {
	query := r.db.WithContext(ctx).Model(&taskModel{})
	if f.ProjectID != "" {
		query = query.Where("project_id = ?", f.ProjectID)
	}
	if f.WorkID != "" {
		query = query.Where("work_id = ?", f.WorkID)
	}
	if f.CreativeWorkID != "" {
		query = query.Where("creative_work_id = ?", f.CreativeWorkID)
	}
	if f.Status != "" {
		query = query.Where("status = ?", f.Status)
	}
	if f.Type != "" {
		query = query.Where("type = ?", f.Type)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("统计任务失败: %w", err)
	}

	page, pageSize := f.Page, f.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}

	var models []taskModel
	if err := query.Order("created_at DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&models).Error; err != nil {
		return nil, 0, fmt.Errorf("查询任务失败: %w", err)
	}
	out := make([]domain.Task, 0, len(models))
	for _, m := range models {
		t, err := toDomainTask(m)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, t)
	}
	return out, total, nil
}

// ClaimNext 原子领取一个待执行任务。
//
// 用 FOR UPDATE SKIP LOCKED 让多个 worker 可以并行领取而不互相阻塞；
// 领取时即把状态改为 RUNNING 并累加 attempts，保证同一任务不会被两个 worker 同时拿到。
// 没有待执行任务时返回 (nil, nil)。
func (r *TaskRepo) ClaimNext(ctx context.Context) (*domain.Task, error) {
	const claimSQL = `
		UPDATE tasks SET
			status = 'RUNNING',
			started_at = now(),
			updated_at = now(),
			attempts = attempts + 1,
			progress = 0,
			progress_message = '开始执行',
			error = ''
		WHERE id = (
			SELECT id FROM tasks
			WHERE status = 'PENDING' AND deleted_at IS NULL
			  AND (next_run_at IS NULL OR next_run_at <= now())
			ORDER BY next_run_at NULLS FIRST, created_at ASC
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)
		RETURNING *`

	var m taskModel
	res := r.db.WithContext(ctx).Raw(claimSQL).Scan(&m)
	if res.Error != nil {
		return nil, fmt.Errorf("领取任务失败: %w", res.Error)
	}
	if m.ID == "" {
		return nil, nil // 队列为空
	}
	t, err := toDomainTask(m)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// UpdateProgress 更新进度。
func (r *TaskRepo) UpdateProgress(ctx context.Context, id string, progress int, message string) error {
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}
	res := r.db.WithContext(ctx).Model(&taskModel{}).Where("id = ?", id).Updates(map[string]any{
		"progress":         progress,
		"progress_message": message,
		"updated_at":       time.Now().UTC(),
	})
	if res.Error != nil {
		return fmt.Errorf("更新任务进度失败: %w", res.Error)
	}
	return nil
}

// Complete 标记任务完成。
func (r *TaskRepo) Complete(ctx context.Context, id string, output map[string]any) error {
	raw, err := json.Marshal(nonNilMap(output))
	if err != nil {
		return fmt.Errorf("序列化任务结果失败: %w", err)
	}
	now := time.Now().UTC()
	res := r.db.WithContext(ctx).Model(&taskModel{}).Where("id = ?", id).Updates(map[string]any{
		"status": string(domain.TaskCompleted), "progress": 100, "progress_message": "已完成",
		"output": string(raw), "error": "", "finished_at": now, "updated_at": now,
	})
	if res.Error != nil {
		return fmt.Errorf("更新任务状态失败: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.ErrTaskNotFound
	}
	return nil
}

// Fail 记录任务失败；若还有重试次数则回到 PENDING（等待再次领取），否则置 FAILED。
func (r *TaskRepo) Fail(ctx context.Context, id string, errMsg string) (domain.TaskStatus, error) {
	var m taskModel
	if err := r.db.WithContext(ctx).First(&m, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", domain.ErrTaskNotFound
		}
		return "", fmt.Errorf("查询任务失败: %w", err)
	}

	now := time.Now().UTC()
	next := domain.TaskRunning
	updates := map[string]any{
		"error": errMsg, "updated_at": now,
	}
	if m.Attempts < m.MaxAttempts {
		// 还能重试：回到队列，并按尝试次数做指数退避（2s / 4s / 8s…，上限 60s）。
		// 没有退避时，确定性失败会在 worker 的 2 秒轮询里被瞬间烧完次数，
		// 瞬时故障（限流、网络抖动）则会在同一秒内反复打上游。
		next = domain.TaskPending
		backoff := retryBackoff(m.Attempts)
		updates["status"] = string(domain.TaskPending)
		updates["started_at"] = nil
		updates["next_run_at"] = now.Add(backoff)
		updates["progress_message"] = fmt.Sprintf("第 %d 次尝试失败，%s 后重试", m.Attempts, backoff)
	} else {
		next = domain.TaskFailed
		updates["status"] = string(domain.TaskFailed)
		updates["progress_message"] = "已失败"
		updates["finished_at"] = now
	}
	if err := r.db.WithContext(ctx).Model(&taskModel{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return "", fmt.Errorf("记录任务失败状态失败: %w", err)
	}
	return next, nil
}

// retryBackoff 返回第 attempts 次失败后的退避时长：2^attempts 秒，封顶 60 秒。
func retryBackoff(attempts int) time.Duration {
	if attempts < 1 {
		attempts = 1
	}
	seconds := 1 << uint(attempts) // 2, 4, 8, 16, 32, 64…
	if seconds > 60 {
		seconds = 60
	}
	return time.Duration(seconds) * time.Second
}

// RequeueStale 把"卡在 RUNNING 且超过阈值没有推进"的任务放回队列，返回回收数量。
//
// 为什么必须有：worker 崩溃、进程被 kill、机器重启都会让 RUNNING 任务永远留在 RUNNING，
// 既不会被执行、也不会进重试，界面上就是一个"永远 30% 的任务"。
func (r *TaskRepo) RequeueStale(ctx context.Context, staleAfter time.Duration) (int, error) {
	if staleAfter <= 0 {
		staleAfter = 30 * time.Minute
	}
	now := time.Now().UTC()
	deadline := now.Add(-staleAfter)
	res := r.db.WithContext(ctx).Model(&taskModel{}).
		Where("status = ? AND deleted_at IS NULL AND COALESCE(started_at, updated_at) < ?", string(domain.TaskRunning), deadline).
		Updates(map[string]any{
			"status":           string(domain.TaskPending),
			"started_at":       nil,
			"next_run_at":      now,
			"progress_message": "任务超时未完成，已放回队列重试",
			"updated_at":       now,
		})
	if res.Error != nil {
		return 0, fmt.Errorf("回收僵死任务失败: %w", res.Error)
	}
	return int(res.RowsAffected), nil
}

// Cancel 取消任务。
func (r *TaskRepo) Cancel(ctx context.Context, id string) error {
	now := time.Now().UTC()
	res := r.db.WithContext(ctx).Model(&taskModel{}).
		Where("id = ? AND status IN ('PENDING','RUNNING','PAUSED')", id).
		Updates(map[string]any{
			"status": string(domain.TaskCancelled), "progress_message": "已取消",
			"finished_at": now, "updated_at": now,
		})
	if res.Error != nil {
		return fmt.Errorf("取消任务失败: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		// 要么任务不存在，要么状态不允许取消 —— 分开报错更清楚
		if _, err := r.GetByID(ctx, id); err != nil {
			return err
		}
		return domain.ErrTaskNotCancellable
	}
	return nil
}

// Retry 重试任务（重置为待执行，并清零已尝试次数）。
func (r *TaskRepo) Retry(ctx context.Context, id string) error {
	now := time.Now().UTC()
	res := r.db.WithContext(ctx).Model(&taskModel{}).
		Where("id = ? AND status IN ('FAILED','CANCELLED')", id).
		Updates(map[string]any{
			"status": string(domain.TaskPending), "progress": 0, "progress_message": "已重新排队",
			"error": "", "attempts": 0, "started_at": nil, "finished_at": nil, "updated_at": now,
		})
	if res.Error != nil {
		return fmt.Errorf("重试任务失败: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		if _, err := r.GetByID(ctx, id); err != nil {
			return err
		}
		return domain.ErrTaskNotRetryable
	}
	return nil
}

// IsCancelled 供 worker 在长任务中途检查是否被取消。
func (r *TaskRepo) IsCancelled(ctx context.Context, id string) (bool, error) {
	var status string
	err := r.db.WithContext(ctx).Model(&taskModel{}).Where("id = ?", id).Select("status").Scan(&status).Error
	if err != nil {
		return false, fmt.Errorf("查询任务状态失败: %w", err)
	}
	if status == "" {
		return false, domain.ErrTaskNotFound
	}
	return status == string(domain.TaskCancelled), nil
}

func toTaskModel(t *domain.Task) (taskModel, error) {
	input, err := json.Marshal(nonNilMap(t.Input))
	if err != nil {
		return taskModel{}, fmt.Errorf("序列化任务入参失败: %w", err)
	}
	output, err := json.Marshal(nonNilMap(t.Output))
	if err != nil {
		return taskModel{}, fmt.Errorf("序列化任务结果失败: %w", err)
	}
	return taskModel{
		ID: t.ID, ProjectID: t.ProjectID, WorkID: t.WorkID, CreativeWorkID: t.CreativeWorkID, Type: t.Type,
		Status: string(t.Status), Progress: t.Progress, ProgressMessage: t.ProgressMessage,
		Input: string(input), Output: string(output), Error: t.Error,
		Attempts: t.Attempts, MaxAttempts: t.MaxAttempts,
		CreatedAt: t.CreatedAt, StartedAt: t.StartedAt, FinishedAt: t.FinishedAt, UpdatedAt: t.UpdatedAt,
	}, nil
}

func toDomainTask(m taskModel) (domain.Task, error) {
	t := domain.Task{
		ID: m.ID, ProjectID: m.ProjectID, WorkID: m.WorkID, CreativeWorkID: m.CreativeWorkID, Type: m.Type,
		Status: domain.TaskStatus(m.Status), Progress: m.Progress, ProgressMessage: m.ProgressMessage,
		Error: m.Error, Attempts: m.Attempts, MaxAttempts: m.MaxAttempts,
		CreatedAt: m.CreatedAt, StartedAt: m.StartedAt, FinishedAt: m.FinishedAt, UpdatedAt: m.UpdatedAt,
		Input: map[string]any{}, Output: map[string]any{},
	}
	if m.Input != "" {
		if err := json.Unmarshal([]byte(m.Input), &t.Input); err != nil {
			return domain.Task{}, fmt.Errorf("解析任务入参失败: %w", err)
		}
	}
	if m.Output != "" {
		if err := json.Unmarshal([]byte(m.Output), &t.Output); err != nil {
			return domain.Task{}, fmt.Errorf("解析任务结果失败: %w", err)
		}
	}
	if m.DeletedAt.Valid {
		ts := m.DeletedAt.Time
		t.DeletedAt = &ts
	}
	return t, nil
}

func nonNilMap(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	return m
}
