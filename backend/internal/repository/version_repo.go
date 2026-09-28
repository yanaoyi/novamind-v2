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

type entityVersionModel struct {
	ID             string    `gorm:"column:id;type:uuid;primaryKey"`
	EntityType     string    `gorm:"column:entity_type;size:40;not null"`
	EntityID       string    `gorm:"column:entity_id;type:uuid;not null"`
	CreativeWorkID string    `gorm:"column:creative_work_id;type:uuid;not null"`
	VersionNo      int       `gorm:"column:version_no;not null"`
	Payload        string    `gorm:"column:payload;type:jsonb;not null;default:'{}'"`
	Note           string    `gorm:"column:note;size:200;not null;default:''"`
	CreatedAt      time.Time `gorm:"column:created_at;not null"`
}

func (entityVersionModel) TableName() string { return "entity_versions" }

// EntityVersionRepo 是版本快照仓储。
type EntityVersionRepo struct {
	db *gorm.DB
}

// NewEntityVersionRepo 构建仓储。
func NewEntityVersionRepo(db *gorm.DB) *EntityVersionRepo { return &EntityVersionRepo{db: db} }

// NextVersionNo 取该实体的下一个版本号。
func (r *EntityVersionRepo) NextVersionNo(ctx context.Context, t domain.EntityVersionType, entityID string) (int, error) {
	var maxNo *int
	err := r.db.WithContext(ctx).Model(&entityVersionModel{}).
		Where("entity_type = ? AND entity_id = ?", string(t), entityID).
		Select("max(version_no)").Scan(&maxNo).Error
	if err != nil {
		return 0, fmt.Errorf("查询版本号失败: %w", err)
	}
	if maxNo == nil {
		return 1, nil
	}
	return *maxNo + 1, nil
}

// Create 写入一份快照。
func (r *EntityVersionRepo) Create(ctx context.Context, v *domain.EntityVersion) error {
	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("生成版本 ID 失败: %w", err)
	}
	v.ID = id.String()
	if v.CreatedAt.IsZero() {
		v.CreatedAt = time.Now().UTC()
	}
	payload, err := json.Marshal(nonNilMap(v.Payload))
	if err != nil {
		return fmt.Errorf("序列化快照失败: %w", err)
	}
	m := entityVersionModel{
		ID: v.ID, EntityType: string(v.EntityType), EntityID: v.EntityID,
		CreativeWorkID: v.CreativeWorkID, VersionNo: v.VersionNo,
		Payload: string(payload), Note: v.Note, CreatedAt: v.CreatedAt,
	}
	if err := r.db.WithContext(ctx).Create(&m).Error; err != nil {
		if isDuplicateKey(err) {
			return errors.New("版本号冲突，请重试")
		}
		return fmt.Errorf("写入版本失败: %w", err)
	}
	return nil
}

// List 列出某实体的所有版本（不含 payload，列表不需要几个 KB 的正文）。
func (r *EntityVersionRepo) List(ctx context.Context, t domain.EntityVersionType, entityID string) ([]domain.EntityVersion, error) {
	var models []entityVersionModel
	if err := r.db.WithContext(ctx).
		Where("entity_type = ? AND entity_id = ?", string(t), entityID).
		Order("version_no DESC").Find(&models).Error; err != nil {
		return nil, fmt.Errorf("查询版本列表失败: %w", err)
	}
	out := make([]domain.EntityVersion, 0, len(models))
	for _, m := range models {
		item := domain.EntityVersion{
			ID: m.ID, EntityType: domain.EntityVersionType(m.EntityType), EntityID: m.EntityID,
			CreativeWorkID: m.CreativeWorkID, VersionNo: m.VersionNo, Note: m.Note, CreatedAt: m.CreatedAt,
		}
		out = append(out, item)
	}
	return out, nil
}

// GetByNo 取某个版本（含 payload）。
func (r *EntityVersionRepo) GetByNo(ctx context.Context, t domain.EntityVersionType, entityID string, no int) (*domain.EntityVersion, error) {
	if no <= 0 {
		return nil, domain.ErrVersionNoInvalid
	}
	var m entityVersionModel
	err := r.db.WithContext(ctx).
		Where("entity_type = ? AND entity_id = ? AND version_no = ?", string(t), entityID, no).
		First(&m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrVersionNotFound
		}
		return nil, fmt.Errorf("查询版本失败: %w", err)
	}
	payload := map[string]any{}
	if m.Payload != "" {
		if err := json.Unmarshal([]byte(m.Payload), &payload); err != nil {
			return nil, fmt.Errorf("解析快照失败: %w", err)
		}
	}
	return &domain.EntityVersion{
		ID: m.ID, EntityType: domain.EntityVersionType(m.EntityType), EntityID: m.EntityID,
		CreativeWorkID: m.CreativeWorkID, VersionNo: m.VersionNo,
		Payload: payload, Note: m.Note, CreatedAt: m.CreatedAt,
	}, nil
}

// Latest 取最新一份版本（用于"和上一版比是否真变了"的去重判断）。
func (r *EntityVersionRepo) Latest(ctx context.Context, t domain.EntityVersionType, entityID string) (*domain.EntityVersion, error) {
	var m entityVersionModel
	err := r.db.WithContext(ctx).
		Where("entity_type = ? AND entity_id = ?", string(t), entityID).
		Order("version_no DESC").First(&m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("查询最新版本失败: %w", err)
	}
	payload := map[string]any{}
	if m.Payload != "" {
		if err := json.Unmarshal([]byte(m.Payload), &payload); err != nil {
			return nil, fmt.Errorf("解析快照失败: %w", err)
		}
	}
	return &domain.EntityVersion{
		ID: m.ID, EntityType: domain.EntityVersionType(m.EntityType), EntityID: m.EntityID,
		CreativeWorkID: m.CreativeWorkID, VersionNo: m.VersionNo, Payload: payload,
		Note: m.Note, CreatedAt: m.CreatedAt,
	}, nil
}
