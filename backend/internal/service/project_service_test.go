package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/yanaoyi/novamindv2/backend/internal/domain"
	"github.com/yanaoyi/novamindv2/backend/internal/repository"
)

// fakeProjectRepo 是内存替身，用于纯业务逻辑测试（不碰数据库）。
type fakeProjectRepo struct {
	items map[string]domain.Project
}

func newFakeRepo() *fakeProjectRepo { return &fakeProjectRepo{items: map[string]domain.Project{}} }

func (f *fakeProjectRepo) Create(_ context.Context, p *domain.Project) error {
	if p.ID == "" {
		p.ID = uuid.NewString()
	}
	f.items[p.ID] = *p
	return nil
}

func (f *fakeProjectRepo) GetByID(_ context.Context, id string) (*domain.Project, error) {
	p, ok := f.items[id]
	if !ok {
		return nil, domain.ErrProjectNotFound
	}
	return &p, nil
}

func (f *fakeProjectRepo) List(_ context.Context, filter repository.ProjectFilter) ([]domain.Project, int64, error) {
	out := make([]domain.Project, 0, len(f.items))
	for _, p := range f.items {
		if filter.Type != nil && p.Type != *filter.Type {
			continue
		}
		out = append(out, p)
	}
	return out, int64(len(out)), nil
}

func (f *fakeProjectRepo) Update(_ context.Context, p *domain.Project) error {
	if _, ok := f.items[p.ID]; !ok {
		return domain.ErrProjectNotFound
	}
	f.items[p.ID] = *p
	return nil
}

func (f *fakeProjectRepo) SoftDelete(_ context.Context, id string) error {
	if _, ok := f.items[id]; !ok {
		return domain.ErrProjectNotFound
	}
	delete(f.items, id)
	return nil
}

func TestProjectServiceCreateNormalizesAndValidates(t *testing.T) {
	svc := NewProjectService(newFakeRepo())
	ctx := context.Background()

	p, err := svc.Create(ctx, CreateProjectInput{
		Name:        "  人间真相  ",
		Description: "  描述  ",
		Type:        domain.ProjectTypeCreative,
	})
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if p.Name != "人间真相" || p.Description != "描述" {
		t.Fatalf("未清洗输入: %+v", p)
	}
	if p.Status != domain.ProjectStatusActive {
		t.Fatalf("未补默认状态: %q", p.Status)
	}
	if p.ID == "" {
		t.Fatal("未生成 ID")
	}

	if _, err := svc.Create(ctx, CreateProjectInput{Name: "  ", Type: domain.ProjectTypeOriginal}); !errors.Is(err, domain.ErrProjectNameRequired) {
		t.Fatalf("空名称应被拒绝，实际 %v", err)
	}
	if _, err := svc.Create(ctx, CreateProjectInput{Name: "x", Type: "BAD"}); !errors.Is(err, domain.ErrProjectTypeInvalid) {
		t.Fatalf("非法类型应被拒绝，实际 %v", err)
	}
}

func TestProjectServiceUpdateKeepsTypeImmutable(t *testing.T) {
	repo := newFakeRepo()
	svc := NewProjectService(repo)
	ctx := context.Background()

	created, err := svc.Create(ctx, CreateProjectInput{Name: "原著", Type: domain.ProjectTypeOriginal})
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}

	newName := "原著（改名）"
	archived := domain.ProjectStatusArchived
	updated, err := svc.Update(ctx, created.ID, UpdateProjectInput{Name: &newName, Status: &archived})
	if err != nil {
		t.Fatalf("更新失败: %v", err)
	}
	if updated.Name != newName || updated.Status != archived {
		t.Fatalf("更新未生效: %+v", updated)
	}
	if updated.Type != domain.ProjectTypeOriginal {
		t.Fatalf("类型不应被改变: %q", updated.Type)
	}

	// 更新为非法状态
	bad := domain.ProjectStatus("ZOMBIE")
	if _, err := svc.Update(ctx, created.ID, UpdateProjectInput{Status: &bad}); !errors.Is(err, domain.ErrProjectStatusBad) {
		t.Fatalf("非法状态应被拒绝，实际 %v", err)
	}

	// 更新不存在的文章
	ghost := "不存在"
	if _, err := svc.Update(ctx, uuid.NewString(), UpdateProjectInput{Name: &ghost}); !errors.Is(err, domain.ErrProjectNotFound) {
		t.Fatalf("不存在的文章应返回 ErrProjectNotFound，实际 %v", err)
	}
}

func TestProjectServiceGetAndDelete(t *testing.T) {
	svc := NewProjectService(newFakeRepo())
	ctx := context.Background()

	created, err := svc.Create(ctx, CreateProjectInput{Name: "待删文章", Type: domain.ProjectTypeOriginal})
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if _, err := svc.Get(ctx, created.ID); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if _, err := svc.Get(ctx, "  "); !errors.Is(err, domain.ErrProjectNotFound) {
		t.Fatalf("空 ID 应返回 ErrProjectNotFound，实际 %v", err)
	}
	if err := svc.Delete(ctx, created.ID); err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	if _, err := svc.Get(ctx, created.ID); !errors.Is(err, domain.ErrProjectNotFound) {
		t.Fatalf("删除后不应查到，实际 %v", err)
	}
}

func TestProjectServiceListReturnsEmptySlice(t *testing.T) {
	svc := NewProjectService(newFakeRepo())
	items, total, err := svc.List(context.Background(), repository.ProjectFilter{})
	if err != nil {
		t.Fatalf("列表查询失败: %v", err)
	}
	if total != 0 {
		t.Fatalf("空库总数应为 0，实际 %d", total)
	}
	if items == nil {
		t.Fatal("items 不应为 nil（应为空数组，避免前端拿到 null）")
	}
}
