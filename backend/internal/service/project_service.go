// Package service 负责业务编排。
// 约束：不直接依赖 HTTP（不 import gin）；数据访问一律通过 repository。
package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/yanaoyi/novamindv2/backend/internal/domain"
	"github.com/yanaoyi/novamindv2/backend/internal/repository"
)

// ProjectRepository 是 service 需要的仓储能力（接口便于测试替身）。
type ProjectRepository interface {
	Create(ctx context.Context, p *domain.Project) error
	GetByID(ctx context.Context, id string) (*domain.Project, error)
	List(ctx context.Context, f repository.ProjectFilter) ([]domain.Project, int64, error)
	Update(ctx context.Context, p *domain.Project) error
	SoftDelete(ctx context.Context, id string) error
}

// CreateProjectInput 是创建文章的入参。
type CreateProjectInput struct {
	Name        string
	Description string
	Type        domain.ProjectType
}

// UpdateProjectInput 是更新文章的入参（nil 表示该字段不变）。
// 注意：Type 不在其中——文章类型创建后不可变更（原著/二创不可互转）。
type UpdateProjectInput struct {
	Name        *string
	Description *string
	Status      *domain.ProjectStatus
}

// ProjectService 是文章业务服务。
type ProjectService struct {
	repo ProjectRepository
}

// NewProjectService 构建服务。
func NewProjectService(repo ProjectRepository) *ProjectService {
	return &ProjectService{repo: repo}
}

// Create 创建文章：清洗 → 校验 → 落库。
func (s *ProjectService) Create(ctx context.Context, in CreateProjectInput) (*domain.Project, error) {
	p := &domain.Project{
		Name:        in.Name,
		Description: in.Description,
		Type:        in.Type,
	}
	p.Normalize()
	if err := p.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

// Get 按 ID 获取文章。
func (s *ProjectService) Get(ctx context.Context, id string) (*domain.Project, error) {
	if strings.TrimSpace(id) == "" {
		return nil, domain.ErrProjectNotFound
	}
	return s.repo.GetByID(ctx, id)
}

// List 分页查询文章。
func (s *ProjectService) List(ctx context.Context, f repository.ProjectFilter) ([]domain.Project, int64, error) {
	items, total, err := s.repo.List(ctx, f)
	if err != nil {
		return nil, 0, fmt.Errorf("查询文章列表失败: %w", err)
	}
	if items == nil {
		items = []domain.Project{}
	}
	return items, total, nil
}

// Update 更新文章的可变字段。
func (s *ProjectService) Update(ctx context.Context, id string, in UpdateProjectInput) (*domain.Project, error) {
	p, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if in.Name != nil {
		p.Name = *in.Name
	}
	if in.Description != nil {
		p.Description = *in.Description
	}
	if in.Status != nil {
		p.Status = *in.Status
	}
	p.Normalize()
	if err := p.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

// Delete 软删除文章。
func (s *ProjectService) Delete(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return domain.ErrProjectNotFound
	}
	return s.repo.SoftDelete(ctx, id)
}
