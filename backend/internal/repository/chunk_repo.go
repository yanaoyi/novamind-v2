package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/yanaoyi/novamindv2/backend/internal/domain"
	"github.com/yanaoyi/novamindv2/backend/internal/retrieval"
)

type chunkModel struct {
	ID          string    `gorm:"column:id;type:uuid;primaryKey"`
	OwnerUserID string    `gorm:"column:owner_user_id;type:uuid;not null"`
	WorkKind    string    `gorm:"column:work_kind;not null"`
	WorkID      string    `gorm:"column:work_id;type:uuid;not null"`
	ChapterID   *string   `gorm:"column:chapter_id;type:uuid"`
	RefKind     string    `gorm:"column:ref_kind;not null"`
	RefID       *string   `gorm:"column:ref_id;type:uuid"`
	Seq         int       `gorm:"column:seq;not null"`
	Content     string    `gorm:"column:content;not null"`
	TokenCount  int       `gorm:"column:token_count;not null"`
	CreatedAt   time.Time `gorm:"column:created_at;not null"`
}

func (chunkModel) TableName() string { return "chunks" }

// ChunkRepo 是检索分块仓储（Phase 9 §9.1）。
type ChunkRepo struct {
	db *gorm.DB
}

// NewChunkRepo 构建仓储。
func NewChunkRepo(db *gorm.DB) *ChunkRepo { return &ChunkRepo{db: db} }

// ReplaceChunks 幂等地替换某个来源（章节 / 大纲节点 / 世界规则 / 记忆事实…）的全部分块。
//
// "先删后建 + 同一事务"是任务书要求：章节改一次就重切一次，绝不能出现
// "旧块还在、新块又进来"导致的重复召回。chunks 为空时等价于删除该来源的全部分块
// （内容被清空或来源被删除时应当如此）。
func (r *ChunkRepo) ReplaceChunks(
	ctx context.Context,
	ownerUserID, workKind, workID, refKind string,
	refID *string,
	chapterID *string,
	chunks []domain.RetrievalChunk,
) error {
	now := time.Now().UTC()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		del := tx.Where("work_kind = ? AND work_id = ? AND ref_kind = ?", workKind, workID, refKind)
		if refID != nil {
			del = del.Where("ref_id = ?", *refID)
		} else {
			del = del.Where("ref_id IS NULL")
		}
		if err := del.Delete(&chunkModel{}).Error; err != nil {
			return fmt.Errorf("清理旧分块失败: %w", err)
		}
		for _, c := range chunks {
			id, err := uuid.NewV7()
			if err != nil {
				return fmt.Errorf("生成分块 ID 失败: %w", err)
			}
			model := chunkModel{
				ID: id.String(), OwnerUserID: ownerUserID, WorkKind: workKind, WorkID: workID,
				ChapterID: chapterID, RefKind: refKind, RefID: refID, Seq: c.Seq,
				Content: c.Content, TokenCount: c.TokenCount, CreatedAt: now,
			}
			if err := tx.Create(&model).Error; err != nil {
				if isForeignKeyViolation(err) {
					return domain.ErrUserNotFound
				}
				return fmt.Errorf("写入分块失败: %w", err)
			}
		}
		return nil
	})
}

// PruneChunks 删掉该 (作品, 来源类型) 下不在 keepRefIDs 里的旧块。
//
// 用途：全量重建时只 Replace 了现存来源，被删掉的章节/规则/大纲节点如果不剪，
// 旧块会永远留在索引里，检索会召回已经不存在的设定。
// ref_id IS NULL 的孤儿行一并清掉（它不对应任何现存来源）。
func (r *ChunkRepo) PruneChunks(
	ctx context.Context,
	ownerUserID, workKind, workID, refKind string,
	keepRefIDs []string,
) error {
	query := r.db.WithContext(ctx).
		Where("work_kind = ? AND work_id = ? AND ref_kind = ?", workKind, workID, refKind)
	if ownerUserID != "" {
		query = query.Where("owner_user_id = ?", ownerUserID)
	}
	if len(keepRefIDs) == 0 {
		query = query.Where("1 = 1")
	} else {
		query = query.Where("ref_id IS NULL OR ref_id::text NOT IN ?", keepRefIDs)
	}
	if err := query.Delete(&chunkModel{}).Error; err != nil {
		return fmt.Errorf("清理过期分块失败: %w", err)
	}
	return nil
}

// CountByWork 统计某个作品当前的分块数（冒烟与排障用）。
func (r *ChunkRepo) CountByWork(ctx context.Context, workKind, workID string) (int, error) {
	var n int64
	if err := r.db.WithContext(ctx).Model(&chunkModel{}).
		Where("work_kind = ? AND work_id = ?", workKind, workID).Count(&n).Error; err != nil {
		return 0, fmt.Errorf("统计分块失败: %w", err)
	}
	return int(n), nil
}

// ListChunks 实现 retrieval.ChunkSource：取出某作品的全部分块供 BM25 打分。
//
// owner 隔离：ownerUserID 非空时只取该用户名下的块（admin/运维排查可传空串表示不过滤）。
// BM25 需要全量打分，所以这里不分页；单作品万级 chunk 在内存里算完全可接受
// （任务书也认可这个量级；真到十万级再加 ivfflat 与分页裁剪）。
func (r *ChunkRepo) ListChunks(
	ctx context.Context,
	ownerUserID, workKind, workID string,
) ([]retrieval.IndexedChunk, error) {
	query := r.db.WithContext(ctx).Model(&chunkModel{}).
		Where("work_kind = ? AND work_id = ?", workKind, workID)
	if ownerUserID != "" {
		query = query.Where("owner_user_id = ?", ownerUserID)
	}
	var models []chunkModel
	if err := query.Order("ref_kind ASC, ref_id ASC, seq ASC").Find(&models).Error; err != nil {
		return nil, fmt.Errorf("查询分块失败: %w", err)
	}
	out := make([]retrieval.IndexedChunk, 0, len(models))
	for _, m := range models {
		out = append(out, retrieval.IndexedChunk{
			ID: m.ID, RefKind: m.RefKind, RefID: m.RefID, Seq: m.Seq, Content: m.Content,
		})
	}
	return out, nil
}
