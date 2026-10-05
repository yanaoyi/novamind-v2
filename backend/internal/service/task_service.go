package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/yanaoyi/novamindv2/backend/internal/domain"
	"github.com/yanaoyi/novamindv2/backend/internal/repository"
)

// TaskRepository 是任务 service 需要的仓储能力。
type TaskRepository interface {
	Create(ctx context.Context, t *domain.Task) error
	GetByID(ctx context.Context, id string) (*domain.Task, error)
	List(ctx context.Context, f repository.TaskFilter) ([]domain.Task, int64, error)
	Cancel(ctx context.Context, id string) error
	Retry(ctx context.Context, id string) error
}

// TaskTypeChecker 判断任务类型是否已注册（由 main 注入，避免 service 依赖 task 包造成循环引用）。
type TaskTypeChecker func(taskType string) bool

// TaskService 负责任务入队与状态管理。
type TaskService struct {
	repo    TaskRepository
	works   WorkLookup
	hasType TaskTypeChecker
	// indexDedup 折叠"同一来源被连续触发"的索引入队（见 index_trigger.go）。
	indexDedup *indexDedup
}

// NewTaskService 构建服务。
func NewTaskService(repo TaskRepository, works WorkLookup, hasType TaskTypeChecker) *TaskService {
	return &TaskService{repo: repo, works: works, hasType: hasType, indexDedup: newIndexDedup(indexDedupWindow)}
}

// ErrTaskTypeUnknown 表示任务类型没有对应的处理函数。
var ErrTaskTypeUnknown = errors.New("任务类型未注册")

// EnqueueInput 是入队参数。
type EnqueueInput struct {
	Type           string
	ProjectID      *string
	WorkID         *string
	CreativeWorkID *string
	Input          map[string]any
	MaxAttempts    int
}

// Enqueue 把任务放入队列（异步执行，调用方拿 task_id 轮询）。
func (s *TaskService) Enqueue(ctx context.Context, in EnqueueInput) (*domain.Task, error) {
	if s.hasType != nil && !s.hasType(strings.TrimSpace(in.Type)) {
		return nil, fmt.Errorf("%w: %s", ErrTaskTypeUnknown, in.Type)
	}
	t := &domain.Task{
		Type: in.Type, ProjectID: in.ProjectID, WorkID: in.WorkID, CreativeWorkID: in.CreativeWorkID,
		Input: in.Input, MaxAttempts: in.MaxAttempts,
	}
	t.Normalize()
	if err := t.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

// EnqueueOriginalReparse 把「重新解析原著章节」排成异步任务。
func (s *TaskService) EnqueueOriginalReparse(ctx context.Context, workID string) (*domain.Task, error) {
	work, err := s.works.GetWorkByID(ctx, workID)
	if err != nil {
		return nil, err
	}
	projectID := work.ProjectID
	return s.Enqueue(ctx, EnqueueInput{
		Type:      "original_reparse",
		ProjectID: &projectID,
		WorkID:    &work.ID,
		Input:     map[string]any{"work_id": work.ID},
	})
}

// Get 取任务。
func (s *TaskService) Get(ctx context.Context, id string) (*domain.Task, error) {
	if strings.TrimSpace(id) == "" {
		return nil, domain.ErrTaskNotFound
	}
	return s.repo.GetByID(ctx, id)
}

// List 列出任务。
func (s *TaskService) List(ctx context.Context, f repository.TaskFilter) ([]domain.Task, int64, error) {
	items, total, err := s.repo.List(ctx, f)
	if err != nil {
		return nil, 0, err
	}
	if items == nil {
		items = []domain.Task{}
	}
	return items, total, nil
}

// Cancel 取消任务。
func (s *TaskService) Cancel(ctx context.Context, id string) error {
	return s.repo.Cancel(ctx, id)
}

// Retry 重试任务。
func (s *TaskService) Retry(ctx context.Context, id string) (*domain.Task, error) {
	if err := s.repo.Retry(ctx, id); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}
