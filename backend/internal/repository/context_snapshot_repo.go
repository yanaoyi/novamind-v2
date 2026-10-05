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

type contextSnapshotModel struct {
	ID             string    `gorm:"column:id;type:uuid;primaryKey"`
	OwnerUserID    string    `gorm:"column:owner_user_id;type:uuid;not null"`
	CreativeWorkID string    `gorm:"column:creative_work_id;type:uuid;not null"`
	ChapterID      *string   `gorm:"column:chapter_id;type:uuid"`
	Kind           string    `gorm:"column:kind;not null"`
	Snapshot       string    `gorm:"column:snapshot;type:jsonb;not null;default:'{}'"`
	CreatedAt      time.Time `gorm:"column:created_at;not null"`
}

func (contextSnapshotModel) TableName() string { return "context_snapshots" }

// ContextSnapshotRepo 是上下文快照仓储（Phase 9 §9.2.2）。
type ContextSnapshotRepo struct {
	db *gorm.DB
}

// NewContextSnapshotRepo 构建仓储。
func NewContextSnapshotRepo(db *gorm.DB) *ContextSnapshotRepo { return &ContextSnapshotRepo{db: db} }

// Create 写入一条快照（只增不改）。
func (r *ContextSnapshotRepo) Create(ctx context.Context, s *domain.ContextSnapshot) error {
	if !s.Kind.Valid() {
		return fmt.Errorf("%w: %s", domain.ErrSnapshotKindBad, s.Kind)
	}
	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("生成快照 ID 失败: %w", err)
	}
	s.ID = id.String()
	s.CreatedAt = time.Now().UTC()
	raw, err := json.Marshal(nonNilMap(s.Snapshot))
	if err != nil {
		return fmt.Errorf("序列化快照失败: %w", err)
	}
	model := contextSnapshotModel{
		ID: s.ID, OwnerUserID: s.OwnerUserID, CreativeWorkID: s.CreativeWorkID,
		ChapterID: s.ChapterID, Kind: string(s.Kind), Snapshot: string(raw), CreatedAt: s.CreatedAt,
	}
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		return fmt.Errorf("写入快照失败: %w", err)
	}
	return nil
}

// ListByChapter 列出某章节的快照（新到旧，不带 payload）。
func (r *ContextSnapshotRepo) ListByChapter(ctx context.Context, chapterID string) ([]domain.ContextSnapshot, error) {
	if _, err := uuid.Parse(chapterID); err != nil {
		return nil, domain.ErrCreativeChapterNotFound
	}
	var models []contextSnapshotModel
	if err := r.db.WithContext(ctx).Where("chapter_id = ?", chapterID).
		Order("created_at DESC").Find(&models).Error; err != nil {
		return nil, fmt.Errorf("查询快照失败: %w", err)
	}
	out := make([]domain.ContextSnapshot, 0, len(models))
	for _, m := range models {
		out = append(out, toDomainSnapshot(m, false))
	}
	return out, nil
}

// GetByID 取快照详情（含 payload）。
func (r *ContextSnapshotRepo) GetByID(ctx context.Context, id string) (*domain.ContextSnapshot, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, domain.ErrSnapshotNotFound
	}
	var m contextSnapshotModel
	if err := r.db.WithContext(ctx).First(&m, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrSnapshotNotFound
		}
		return nil, fmt.Errorf("查询快照失败: %w", err)
	}
	s := toDomainSnapshot(m, true)
	return &s, nil
}

func toDomainSnapshot(m contextSnapshotModel, withPayload bool) domain.ContextSnapshot {
	out := domain.ContextSnapshot{
		ID: m.ID, OwnerUserID: m.OwnerUserID, CreativeWorkID: m.CreativeWorkID,
		ChapterID: m.ChapterID, Kind: domain.SnapshotKind(m.Kind), CreatedAt: m.CreatedAt,
	}
	if withPayload {
		payload := map[string]any{}
		unmarshalJSONB("context_snapshots", "snapshot", m.Snapshot, &payload)
		out.Snapshot = payload
	}
	return out
}
