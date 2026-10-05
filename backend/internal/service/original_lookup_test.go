package service

import (
	"context"
	"errors"
	"testing"

	"github.com/yanaoyi/novamindv2/backend/internal/domain"
)

// stubOriginalRepo 只实现本组测试用到的方法（其余由嵌入接口兜底）。
type stubOriginalRepo struct {
	OriginalRepository
	byProject map[string]*domain.OriginalWork
}

func (r *stubOriginalRepo) GetWorkByProject(_ context.Context, projectID string) (*domain.OriginalWork, error) {
	if w, ok := r.byProject[projectID]; ok {
		return w, nil
	}
	return nil, domain.ErrOriginalNotFound
}

// 「原著 → 总览」选完文章是按 project 反查原著的（GET /projects/:id/original）。
// 这条接口曾经缺失（只有 POST 创建），导致前端把"查不到"当成"这篇文章还没有原著"，
// workId 永远设不上、后续页面整片不可用。这个测试守住"按文章取原著"的两种结果。
func TestGetByProject(t *testing.T) {
	work := &domain.OriginalWork{ID: "ow-1", ProjectID: "p-1", Title: "王朔文集"}
	svc := NewOriginalService(&stubOriginalRepo{byProject: map[string]*domain.OriginalWork{"p-1": work}}, nil, nil, 0)

	got, err := svc.GetByProject(context.Background(), "p-1")
	if err != nil {
		t.Fatalf("有原著时应返回原著: %v", err)
	}
	if got.ID != "ow-1" {
		t.Errorf("返回了错误的原著: %+v", got)
	}

	if _, err := svc.GetByProject(context.Background(), "p-2"); !errors.Is(err, domain.ErrOriginalNotFound) {
		t.Errorf("该文章没有原著时应返回 ErrOriginalNotFound（前端映射成 null 再引导导入），实际 %v", err)
	}
	if _, err := svc.GetByProject(context.Background(), "  "); !errors.Is(err, domain.ErrOriginalNotFound) {
		t.Errorf("空文章 ID 应直接按未找到处理，实际 %v", err)
	}
}
