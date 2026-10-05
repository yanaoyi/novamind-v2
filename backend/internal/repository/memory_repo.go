package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/yanaoyi/novamindv2/backend/internal/domain"
)

type memoryFactModel struct {
	ID             string    `gorm:"column:id;type:uuid;primaryKey"`
	OwnerUserID    string    `gorm:"column:owner_user_id;type:uuid;not null"`
	CreativeWorkID string    `gorm:"column:creative_work_id;type:uuid;not null"`
	Kind           string    `gorm:"column:kind;not null"`
	Subject        string    `gorm:"column:subject;not null"`
	Fact           string    `gorm:"column:fact;not null"`
	ChapterID      *string   `gorm:"column:chapter_id;type:uuid"`
	SupersededBy   *string   `gorm:"column:superseded_by;type:uuid"`
	CreatedAt      time.Time `gorm:"column:created_at;not null"`
}

func (memoryFactModel) TableName() string { return "memory_facts" }

type chapterSummaryModel struct {
	ChapterID   string    `gorm:"column:chapter_id;type:uuid;primaryKey"`
	OwnerUserID string    `gorm:"column:owner_user_id;type:uuid;not null"`
	Summary     string    `gorm:"column:summary;not null"`
	CreatedAt   time.Time `gorm:"column:created_at;not null"`
}

func (chapterSummaryModel) TableName() string { return "chapter_summaries" }

// FactWrite 是一条待写入的记忆事实（由事实抽取器产出）。
type FactWrite struct {
	Kind      string
	Subject   string
	Fact      string
	ChapterID *string
}

// FactWriteOutcome 是一次写入的结果。
//
// 为什么要带 ID 而不是只给计数：调用方（记忆服务）紧接着要把新事实写进检索索引、
// 把被替代事实的旧索引清掉 —— 没有 ID 就没法精确定位，只能"全量重建"糊过去。
type FactWriteOutcome struct {
	Created    []domain.MemoryFact
	Superseded []string
}

// MemoryRepo 是长篇记忆的仓储（Phase 9 §9.3）。
type MemoryRepo struct {
	db *gorm.DB
}

// NewMemoryRepo 构建仓储。
func NewMemoryRepo(db *gorm.DB) *MemoryRepo { return &MemoryRepo{db: db} }

// CreateFacts 写入一批事实并维护 supersede 语义，返回 (新增条数, 被替代条数)。
//
// 事务内逐条处理：
//  1. 与"同作品 + 同 kind + 同 subject"的当前有效事实比对；文本完全相同则跳过
//     （抽取重复执行是常态，不该长出重复行）；
//  2. 否则插入新事实，并把旧的同类事实的 superseded_by 指向它（只标记不删）——
//     "左臂受伤 → 后来痊愈"因此是可追溯的演变，而不是把历史抹掉。
//
// supersede 的粒度是 (kind, subject)，与任务书 §9.3.1 一致。它偏激进
// （同一人物的不同状态会互相替代），但那正是"状态"的语义；需要并列事实时应换 subject
// （如 "沈砚/左臂" 与 "沈砚/佩刀"）。
func (r *MemoryRepo) CreateFacts(
	ctx context.Context,
	ownerUserID, workID string,
	facts []FactWrite,
) (FactWriteOutcome, error) {
	outcome := FactWriteOutcome{}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, in := range facts {
			fact := domain.MemoryFact{Kind: domain.FactKind(in.Kind), Subject: in.Subject, Fact: in.Fact}
			fact.Normalize()
			if err := fact.Validate(); err != nil {
				return err
			}
			var active []memoryFactModel
			if err := tx.Where(
				"creative_work_id = ? AND kind = ? AND subject = ? AND superseded_by IS NULL",
				workID, string(fact.Kind), fact.Subject,
			).Find(&active).Error; err != nil {
				return fmt.Errorf("查询既有记忆事实失败: %w", err)
			}
			duplicated := false
			for _, a := range active {
				if strings.TrimSpace(a.Fact) == fact.Fact {
					duplicated = true
					break
				}
			}
			if duplicated {
				continue
			}

			id, err := uuid.NewV7()
			if err != nil {
				return fmt.Errorf("生成记忆事实 ID 失败: %w", err)
			}
			row := memoryFactModel{
				ID: id.String(), OwnerUserID: ownerUserID, CreativeWorkID: workID,
				Kind: string(fact.Kind), Subject: fact.Subject, Fact: fact.Fact,
				ChapterID: in.ChapterID, CreatedAt: time.Now().UTC(),
			}
			if err := tx.Create(&row).Error; err != nil {
				if isForeignKeyViolation(err) {
					return domain.ErrUserNotFound
				}
				return fmt.Errorf("写入记忆事实失败: %w", err)
			}
			outcome.Created = append(outcome.Created, domain.MemoryFact{
				ID: row.ID, OwnerUserID: ownerUserID, CreativeWorkID: workID,
				Kind: domain.FactKind(row.Kind), Subject: row.Subject, Fact: row.Fact,
				ChapterID: row.ChapterID, CreatedAt: row.CreatedAt,
			})

			if len(active) > 0 {
				ids := make([]string, 0, len(active))
				for _, a := range active {
					ids = append(ids, a.ID)
				}
				if err := tx.Model(&memoryFactModel{}).
					Where("id IN ? AND superseded_by IS NULL", ids).
					Update("superseded_by", row.ID).Error; err != nil {
					return fmt.Errorf("标记旧事实被替代失败: %w", err)
				}
				outcome.Superseded = append(outcome.Superseded, ids...)
			}
		}
		return nil
	})
	if err != nil {
		return FactWriteOutcome{}, err
	}
	return outcome, nil
}

// ListFacts 列出某部作品的记忆事实（新到旧）。
//
// onlyActive=true 时只返回未被替代的事实 —— 检索与一致性检查要的是"当前成立的事实"；
// 作者追溯演变过程时用 false，能看到包含已被替代的全部行。
func (r *MemoryRepo) ListFacts(ctx context.Context, workID string, onlyActive bool) ([]domain.MemoryFact, error) {
	query := r.db.WithContext(ctx).Model(&memoryFactModel{}).Where("creative_work_id = ?", workID)
	if onlyActive {
		query = query.Where("superseded_by IS NULL")
	}
	var rows []memoryFactModel
	if err := query.Order("created_at DESC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("查询记忆事实失败: %w", err)
	}
	out := make([]domain.MemoryFact, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.MemoryFact{
			ID: row.ID, OwnerUserID: row.OwnerUserID, CreativeWorkID: row.CreativeWorkID,
			Kind: domain.FactKind(row.Kind), Subject: row.Subject, Fact: row.Fact,
			ChapterID: row.ChapterID, SupersededBy: row.SupersededBy, CreatedAt: row.CreatedAt,
		})
	}
	return out, nil
}

// UpsertSummary 写入/覆盖某章摘要（同一章被重复抽取是常态，按主键覆盖）。
func (r *MemoryRepo) UpsertSummary(ctx context.Context, ownerUserID, chapterID, summary string) error {
	summary = strings.TrimSpace(summary)
	if summary == "" {
		return domain.ErrFactTextEmpty
	}
	row := chapterSummaryModel{
		ChapterID: chapterID, OwnerUserID: ownerUserID,
		Summary: summary, CreatedAt: time.Now().UTC(),
	}
	if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "chapter_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"owner_user_id", "summary", "created_at"}),
	}).Create(&row).Error; err != nil {
		if isForeignKeyViolation(err) {
			return domain.ErrCreativeChapterNotFound
		}
		return fmt.Errorf("写入章节摘要失败: %w", err)
	}
	return nil
}

// GetSummary 取某章摘要（不存在时返回 ErrChapterSummaryNotFound）。
func (r *MemoryRepo) GetSummary(ctx context.Context, chapterID string) (*domain.ChapterSummary, error) {
	var row chapterSummaryModel
	err := r.db.WithContext(ctx).Where("chapter_id = ?", chapterID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrChapterSummaryNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("查询章节摘要失败: %w", err)
	}
	return &domain.ChapterSummary{
		ChapterID: row.ChapterID, OwnerUserID: row.OwnerUserID,
		Summary: row.Summary, CreatedAt: row.CreatedAt,
	}, nil
}
