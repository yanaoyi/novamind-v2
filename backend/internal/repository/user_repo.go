package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/yanaoyi/novamindv2/backend/internal/domain"
)

type userModel struct {
	ID          string `gorm:"column:id;type:uuid;primaryKey"`
	Username    string `gorm:"column:username;size:64;not null"`
	DisplayName string `gorm:"column:display_name;size:120;not null;default:''"`
	Role        string `gorm:"column:role;size:20;not null;default:user"`
	Status      string `gorm:"column:status;size:20;not null;default:active"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (userModel) TableName() string { return "users" }

// UserRepo 是用户仓储（Phase 9 最小骨架）。
type UserRepo struct {
	db *gorm.DB
}

// NewUserRepo 构建仓储。
func NewUserRepo(db *gorm.DB) *UserRepo { return &UserRepo{db: db} }

// DefaultUsername 是单用户模式下的默认账号名（迁移 0018 里已建好）。
const DefaultUsername = "default"

// DefaultUserID 取默认账号 ID：Phase 9 的新数据（chunks 等）都挂在它名下。
//
// 将来接入真正的登录后，这个方法会被"当前登录用户"取代；
// 之所以现在就要它，是因为任务书要求 owner_user_id **不可为空**，必须有个确定的归属。
func (r *UserRepo) DefaultUserID(ctx context.Context) (string, error) {
	var m userModel
	if err := r.db.WithContext(ctx).First(&m, "lower(username) = ?", DefaultUsername).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", domain.ErrUserNotFound
		}
		return "", fmt.Errorf("查询默认用户失败: %w", err)
	}
	return m.ID, nil
}

// GetByID 取用户。
func (r *UserRepo) GetByID(ctx context.Context, id string) (*domain.User, error) {
	var m userModel
	if err := r.db.WithContext(ctx).First(&m, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrUserNotFound
		}
		return nil, fmt.Errorf("查询用户失败: %w", err)
	}
	return &domain.User{
		ID: m.ID, Username: m.Username, DisplayName: m.DisplayName,
		Role: domain.UserRole(m.Role), Status: m.Status,
		CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}, nil
}
