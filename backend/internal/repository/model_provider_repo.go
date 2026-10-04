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

type modelProviderModel struct {
	ID           string         `gorm:"column:id;type:uuid;primaryKey"`
	Name         string         `gorm:"column:name;size:120;not null"`
	Provider     string         `gorm:"column:provider;size:40;not null"`
	APIBase      string         `gorm:"column:api_base;size:300;not null"`
	APIKeyCipher string         `gorm:"column:api_key_cipher;not null;default:''"`
	ModelName    string         `gorm:"column:model_name;size:120;not null"`
	Purpose      string         `gorm:"column:purpose;size:20;not null;default:chat"`
	Temperature  float64        `gorm:"column:temperature;type:numeric(3,2);not null;default:0.70"`
	MaxTokens    int            `gorm:"column:max_tokens;not null;default:4096"`
	TimeoutSec   int            `gorm:"column:timeout_sec;not null;default:120"`
	Enabled      bool           `gorm:"column:enabled;not null;default:true"`
	IsDefault    bool           `gorm:"column:is_default;not null;default:false"`
	Notes        string         `gorm:"column:notes;not null;default:''"`
	CreatedAt    time.Time      `gorm:"column:created_at;not null"`
	UpdatedAt    time.Time      `gorm:"column:updated_at;not null"`
	DeletedAt    gorm.DeletedAt `gorm:"column:deleted_at;index"`
}

func (modelProviderModel) TableName() string { return "model_providers" }

// ModelProviderRepo 是模型配置仓储。
type ModelProviderRepo struct {
	db *gorm.DB
}

// NewModelProviderRepo 构建仓储。
func NewModelProviderRepo(db *gorm.DB) *ModelProviderRepo {
	return &ModelProviderRepo{db: db}
}

// Create 新增模型配置；apiKeyCipher 为已加密的密钥。
func (r *ModelProviderRepo) Create(ctx context.Context, p *domain.ModelProvider, apiKeyCipher string) error {
	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("生成模型配置 ID 失败: %w", err)
	}
	p.ID = id.String()
	now := time.Now().UTC()
	p.CreatedAt, p.UpdatedAt = now, now
	p.HasAPIKey = apiKeyCipher != ""

	m := modelProviderModel{
		ID: p.ID, Name: p.Name, Provider: string(p.Provider), APIBase: p.APIBase,
		APIKeyCipher: apiKeyCipher, ModelName: p.ModelName, Purpose: string(p.Purpose),
		Temperature: p.Temperature, MaxTokens: p.MaxTokens, TimeoutSec: p.TimeoutSec,
		Enabled: p.Enabled, IsDefault: p.IsDefault, Notes: p.Notes,
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
	if err := r.db.WithContext(ctx).Create(&m).Error; err != nil {
		if isUniqueViolation(err) {
			return domain.ErrProviderDuplicate
		}
		return fmt.Errorf("创建模型配置失败: %w", err)
	}
	return nil
}

// GetByID 取模型配置（不含密文）。
func (r *ModelProviderRepo) GetByID(ctx context.Context, id string) (*domain.ModelProvider, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, domain.ErrProviderNotFound
	}
	var m modelProviderModel
	if err := r.db.WithContext(ctx).First(&m, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrProviderNotFound
		}
		return nil, fmt.Errorf("查询模型配置失败: %w", err)
	}
	p := toDomainProvider(m)
	return &p, nil
}

// GetKeyCipher 取加密后的密钥（只给 service 解密用）。
func (r *ModelProviderRepo) GetKeyCipher(ctx context.Context, id string) (string, error) {
	if _, err := uuid.Parse(id); err != nil {
		return "", domain.ErrProviderNotFound
	}
	var m modelProviderModel
	if err := r.db.WithContext(ctx).Select("id, api_key_cipher").First(&m, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", domain.ErrProviderNotFound
		}
		return "", fmt.Errorf("查询模型密钥失败: %w", err)
	}
	return m.APIKeyCipher, nil
}

// List 列出模型配置。
func (r *ModelProviderRepo) List(ctx context.Context) ([]domain.ModelProvider, error) {
	var models []modelProviderModel
	if err := r.db.WithContext(ctx).Order("is_default DESC, created_at ASC").Find(&models).Error; err != nil {
		return nil, fmt.Errorf("查询模型配置失败: %w", err)
	}
	out := make([]domain.ModelProvider, 0, len(models))
	for _, m := range models {
		out = append(out, toDomainProvider(m))
	}
	return out, nil
}

// FindChatProvider 返回可用的对话模型配置：优先默认，其次最早创建的启用项。
func (r *ModelProviderRepo) FindChatProvider(ctx context.Context) (*domain.ModelProvider, error) {
	var m modelProviderModel
	err := r.db.WithContext(ctx).
		Where("enabled AND purpose IN ('chat','both')").
		Order("is_default DESC, created_at ASC").First(&m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNoProviderAvailable
		}
		return nil, fmt.Errorf("查询可用模型配置失败: %w", err)
	}
	p := toDomainProvider(m)
	return &p, nil
}

// Update 更新模型配置；apiKeyCipher 为 nil 表示不改密钥。
func (r *ModelProviderRepo) Update(ctx context.Context, p *domain.ModelProvider, apiKeyCipher *string) error {
	now := time.Now().UTC()
	updates := map[string]any{
		"name":        p.Name,
		"provider":    string(p.Provider),
		"api_base":    p.APIBase,
		"model_name":  p.ModelName,
		"purpose":     string(p.Purpose),
		"temperature": p.Temperature,
		"max_tokens":  p.MaxTokens,
		"timeout_sec": p.TimeoutSec,
		"enabled":     p.Enabled,
		"is_default":  p.IsDefault,
		"notes":       p.Notes,
		"updated_at":  now,
	}
	if apiKeyCipher != nil {
		updates["api_key_cipher"] = *apiKeyCipher
	}

	res := r.db.WithContext(ctx).Model(&modelProviderModel{}).Where("id = ?", p.ID).Updates(updates)
	if res.Error != nil {
		if isUniqueViolation(res.Error) {
			return domain.ErrProviderDuplicate
		}
		return fmt.Errorf("更新模型配置失败: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		if _, err := r.GetByID(ctx, p.ID); err != nil {
			return err
		}
	}
	p.UpdatedAt = now
	if apiKeyCipher != nil {
		p.HasAPIKey = *apiKeyCipher != ""
	}
	return nil
}

// SetDefault 把某配置设为指定用途的默认（同用途其它项取消默认）。
func (r *ModelProviderRepo) SetDefault(ctx context.Context, id string, purpose domain.ProviderPurpose) error {
	if _, err := uuid.Parse(id); err != nil {
		return domain.ErrProviderNotFound
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&modelProviderModel{}).
			Where("is_default AND purpose = ? AND id <> ?", string(purpose), id).
			Update("is_default", false).Error; err != nil {
			return fmt.Errorf("清理旧默认项失败: %w", err)
		}
		res := tx.Model(&modelProviderModel{}).Where("id = ?", id).Updates(map[string]any{
			"is_default": true, "updated_at": time.Now().UTC(),
		})
		if res.Error != nil {
			return fmt.Errorf("设置默认模型失败: %w", res.Error)
		}
		if res.RowsAffected == 0 {
			return domain.ErrProviderNotFound
		}
		return nil
	})
}

// Delete 软删除模型配置。
func (r *ModelProviderRepo) Delete(ctx context.Context, id string) error {
	if _, err := uuid.Parse(id); err != nil {
		return domain.ErrProviderNotFound
	}
	// 软删除只标记行，密钥密文会继续留在库里。模型账号属于"删了就必须删干净"的数据，
	// 所以这里先把密文清空（凭证不可恢复），再打软删除标记保留审计痕迹。
	if err := r.db.WithContext(ctx).Model(&modelProviderModel{}).
		Where("id = ?", id).
		Update("api_key_cipher", "").Error; err != nil {
		return fmt.Errorf("清除模型密钥失败: %w", err)
	}
	res := r.db.WithContext(ctx).Where("id = ?", id).Delete(&modelProviderModel{})
	if res.Error != nil {
		return fmt.Errorf("删除模型配置失败: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.ErrProviderNotFound
	}
	return nil
}

func toDomainProvider(m modelProviderModel) domain.ModelProvider {
	p := domain.ModelProvider{
		ID: m.ID, Name: m.Name, Provider: domain.ProviderType(m.Provider), APIBase: m.APIBase,
		ModelName: m.ModelName, Purpose: domain.ProviderPurpose(m.Purpose),
		Temperature: m.Temperature, MaxTokens: m.MaxTokens, TimeoutSec: m.TimeoutSec,
		Enabled: m.Enabled, IsDefault: m.IsDefault, Notes: m.Notes,
		HasAPIKey: m.APIKeyCipher != "",
		CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}
	if m.DeletedAt.Valid {
		t := m.DeletedAt.Time
		p.DeletedAt = &t
	}
	return p
}
