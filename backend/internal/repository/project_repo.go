// Package repository 负责数据访问。
// 约束：只做存取，不写业务判断；对上层暴露 domain 实体，不暴露 GORM 模型。
package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/yanaoyi/novamindv2/backend/internal/domain"
)

// projectModel 是 projects 表的持久化模型（与 domain 实体分离，避免 ORM 污染领域层）。
type projectModel struct {
	ID          string         `gorm:"column:id;type:uuid;primaryKey"`
	Name        string         `gorm:"column:name;size:200;not null"`
	Description string         `gorm:"column:description;not null;default:''"`
	Type        string         `gorm:"column:type;size:20;not null"`
	Status      string         `gorm:"column:status;size:20;not null;default:ACTIVE"`
	CreatedAt   time.Time      `gorm:"column:created_at;not null"`
	UpdatedAt   time.Time      `gorm:"column:updated_at;not null"`
	DeletedAt   gorm.DeletedAt `gorm:"column:deleted_at;index"`
}

// TableName 固定表名。
func (projectModel) TableName() string { return "projects" }

// ProjectFilter 是列表查询条件。
type ProjectFilter struct {
	Type     *domain.ProjectType
	Status   *domain.ProjectStatus
	Keyword  string // 按名称模糊匹配
	Page     int
	PageSize int
}

const (
	defaultPageSize = 20
	maxPageSize     = 100
)

// Normalize 修正分页参数并返回偏移量。
func (f *ProjectFilter) Normalize() (offset, limit int) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 {
		f.PageSize = defaultPageSize
	}
	if f.PageSize > maxPageSize {
		f.PageSize = maxPageSize
	}
	return (f.Page - 1) * f.PageSize, f.PageSize
}

// ProjectRepo 是文章仓储。
type ProjectRepo struct {
	db *gorm.DB
}

// NewProjectRepo 构建仓储。
func NewProjectRepo(db *gorm.DB) *ProjectRepo { return &ProjectRepo{db: db} }

// Create 写入新文章；ID 为空时自动生成 UUIDv7。
func (r *ProjectRepo) Create(ctx context.Context, p *domain.Project) error {
	if p.ID == "" {
		id, err := uuid.NewV7()
		if err != nil {
			return fmt.Errorf("生成项目 ID 失败: %w", err)
		}
		p.ID = id.String()
	}
	now := time.Now().UTC()
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now
	}
	p.UpdatedAt = now

	m := toProjectModel(p)
	if err := r.db.WithContext(ctx).Create(&m).Error; err != nil {
		return fmt.Errorf("创建项目失败: %w", err)
	}
	p.CreatedAt, p.UpdatedAt = m.CreatedAt, m.UpdatedAt
	return nil
}

// GetByID 按 ID 查询未删除的文章。
func (r *ProjectRepo) GetByID(ctx context.Context, id string) (*domain.Project, error) {
	if _, err := uuid.Parse(id); err != nil {
		// 非法 UUID 直接视作不存在，避免把数据库语法错误暴露成 500
		return nil, domain.ErrProjectNotFound
	}
	var m projectModel
	if err := r.db.WithContext(ctx).First(&m, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrProjectNotFound
		}
		return nil, fmt.Errorf("查询项目失败: %w", err)
	}
	p := toDomainProject(m)
	return &p, nil
}

// List 分页查询，返回条目与总数。
func (r *ProjectRepo) List(ctx context.Context, f ProjectFilter) ([]domain.Project, int64, error) {
	query := r.db.WithContext(ctx).Model(&projectModel{})
	if f.Type != nil {
		query = query.Where("type = ?", string(*f.Type))
	}
	if f.Status != nil {
		query = query.Where("status = ?", string(*f.Status))
	}
	if f.Keyword != "" {
		query = query.Where("name ILIKE ?", "%"+f.Keyword+"%")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("统计项目数量失败: %w", err)
	}

	offset, limit := f.Normalize()
	var models []projectModel
	if err := query.Order("created_at DESC, id DESC").Offset(offset).Limit(limit).Find(&models).Error; err != nil {
		return nil, 0, fmt.Errorf("查询项目列表失败: %w", err)
	}

	items := make([]domain.Project, 0, len(models))
	for _, m := range models {
		items = append(items, toDomainProject(m))
	}
	return items, total, nil
}

// Update 更新可变字段（type 不可变，符合"原著/二创不可互转"的产品约束）。
func (r *ProjectRepo) Update(ctx context.Context, p *domain.Project) error {
	p.UpdatedAt = time.Now().UTC()
	res := r.db.WithContext(ctx).Model(&projectModel{}).
		Where("id = ?", p.ID).
		Updates(map[string]any{
			"name":        p.Name,
			"description": p.Description,
			"status":      string(p.Status),
			"updated_at":  p.UpdatedAt,
		})
	if res.Error != nil {
		return fmt.Errorf("更新项目失败: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		// 可能是记录不存在，也可能是字段值未变化；用一次查询判定
		if _, err := r.GetByID(ctx, p.ID); err != nil {
			return err
		}
	}
	return nil
}

// SoftDelete 软删除文章（写 deleted_at，不物理删除）。
func (r *ProjectRepo) SoftDelete(ctx context.Context, id string) error {
	if _, err := uuid.Parse(id); err != nil {
		return domain.ErrProjectNotFound
	}
	res := r.db.WithContext(ctx).Where("id = ?", id).Delete(&projectModel{})
	if res.Error != nil {
		return fmt.Errorf("删除项目失败: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.ErrProjectNotFound
	}
	return nil
}

func toProjectModel(p *domain.Project) projectModel {
	return projectModel{
		ID:          p.ID,
		Name:        p.Name,
		Description: p.Description,
		Type:        string(p.Type),
		Status:      string(p.Status),
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
	}
}

func toDomainProject(m projectModel) domain.Project {
	p := domain.Project{
		ID:          m.ID,
		Name:        m.Name,
		Description: m.Description,
		Type:        domain.ProjectType(m.Type),
		Status:      domain.ProjectStatus(m.Status),
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
	if m.DeletedAt.Valid {
		t := m.DeletedAt.Time
		p.DeletedAt = &t
	}
	return p
}
