package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/yanaoyi/novamindv2/backend/internal/domain"
)

type proposalModel struct {
	ID         string         `gorm:"column:id;type:uuid;primaryKey"`
	WorkID     string         `gorm:"column:work_id;type:uuid;not null"`
	TaskID     *string        `gorm:"column:task_id;type:uuid"`
	Stage      string         `gorm:"column:stage;size:40;not null"`
	EntityType string         `gorm:"column:entity_type;size:40;not null"`
	Title      string         `gorm:"column:title;size:200;not null;default:''"`
	Payload    string         `gorm:"column:payload;type:jsonb;not null;default:'{}'"`
	Evidence   string         `gorm:"column:evidence;not null;default:''"`
	Confidence int            `gorm:"column:confidence;not null;default:0"`
	Status     string         `gorm:"column:status;size:20;not null;default:PENDING"`
	ReviewNote string         `gorm:"column:review_note;not null;default:''"`
	ReviewedAt *time.Time     `gorm:"column:reviewed_at"`
	AppliedID  *string        `gorm:"column:applied_id;type:uuid"`
	CreatedAt  time.Time      `gorm:"column:created_at;not null"`
	UpdatedAt  time.Time      `gorm:"column:updated_at;not null"`
	DeletedAt  gorm.DeletedAt `gorm:"column:deleted_at;index"`
}

func (proposalModel) TableName() string { return "analysis_proposals" }

// ProposalFilter 是提案列表查询条件。
type ProposalFilter struct {
	WorkID     string
	Stage      string
	EntityType string
	Status     string
	Page       int
	PageSize   int
}

// AnalysisProposalRepo 是分析提案仓储。
type AnalysisProposalRepo struct {
	db *gorm.DB
}

// NewAnalysisProposalRepo 构建仓储。
func NewAnalysisProposalRepo(db *gorm.DB) *AnalysisProposalRepo {
	return &AnalysisProposalRepo{db: db}
}

// CreateBatch 批量写入提案；同一任务内同实体的重复提案会被唯一索引挡掉（跳过而不是报错）。
// 返回实际写入条数。
func (r *AnalysisProposalRepo) CreateBatch(ctx context.Context, proposals []domain.AnalysisProposal) (int, error) {
	if len(proposals) == 0 {
		return 0, nil
	}
	models := make([]proposalModel, 0, len(proposals))
	now := time.Now().UTC()
	for i := range proposals {
		p := &proposals[i]
		p.Normalize()
		if err := p.Validate(); err != nil {
			// 单条不合法不该拖垮整批：跳过并继续（任务里会把跳过数写进 output）
			continue
		}
		if p.ID == "" {
			id, err := uuid.NewV7()
			if err != nil {
				return 0, fmt.Errorf("生成提案 ID 失败: %w", err)
			}
			p.ID = id.String()
		}
		if p.CreatedAt.IsZero() {
			p.CreatedAt = now
		}
		p.UpdatedAt = now

		raw, err := json.Marshal(p.Payload)
		if err != nil {
			return 0, fmt.Errorf("序列化提案内容失败: %w", err)
		}
		models = append(models, proposalModel{
			ID: p.ID, WorkID: p.WorkID, TaskID: p.TaskID, Stage: string(p.Stage),
			EntityType: string(p.EntityType), Title: p.Title, Payload: string(raw),
			Evidence: p.Evidence, Confidence: p.Confidence, Status: string(p.Status),
			CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
		})
	}
	if len(models) == 0 {
		return 0, nil
	}

	res := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&models)
	if res.Error != nil {
		return 0, fmt.Errorf("写入提案失败: %w", res.Error)
	}
	return int(res.RowsAffected), nil
}

// GetByID 取提案。
func (r *AnalysisProposalRepo) GetByID(ctx context.Context, id string) (*domain.AnalysisProposal, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, domain.ErrProposalNotFound
	}
	var m proposalModel
	if err := r.db.WithContext(ctx).First(&m, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrProposalNotFound
		}
		return nil, fmt.Errorf("查询提案失败: %w", err)
	}
	p, err := toDomainProposal(m)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// List 分页查询提案。
func (r *AnalysisProposalRepo) List(ctx context.Context, f ProposalFilter) ([]domain.AnalysisProposal, int64, error) {
	query := r.db.WithContext(ctx).Model(&proposalModel{})
	if f.WorkID != "" {
		query = query.Where("work_id = ?", f.WorkID)
	}
	if f.Stage != "" {
		query = query.Where("stage = ?", f.Stage)
	}
	if f.EntityType != "" {
		query = query.Where("entity_type = ?", f.EntityType)
	}
	if f.Status != "" {
		query = query.Where("status = ?", f.Status)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("统计提案失败: %w", err)
	}

	page, pageSize := f.Page, f.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 50
	}
	if pageSize > 200 {
		pageSize = 200
	}

	var models []proposalModel
	if err := query.Order("created_at DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&models).Error; err != nil {
		return nil, 0, fmt.Errorf("查询提案失败: %w", err)
	}
	out := make([]domain.AnalysisProposal, 0, len(models))
	for _, m := range models {
		p, err := toDomainProposal(m)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, p)
	}
	return out, total, nil
}

// CountByStatus 统计某原著各状态的提案数量（界面上的角标）。
func (r *AnalysisProposalRepo) CountByStatus(ctx context.Context, workID string) (pending, approved, rejected int64, err error) {
	// 注意：这里必须用三次独立查询。gorm 的链式调用会复用同一个 statement，
	// 复用变量会导致条件叠加，统计结果恒为 0（冒烟测试抓到过）。
	count := func(status domain.ProposalStatus) (int64, error) {
		var n int64
		err := r.db.WithContext(ctx).Model(&proposalModel{}).
			Where("work_id = ? AND status = ?", workID, string(status)).
			Count(&n).Error
		return n, err
	}
	if pending, err = count(domain.ProposalPending); err != nil {
		return 0, 0, 0, fmt.Errorf("统计待审提案失败: %w", err)
	}
	if approved, err = count(domain.ProposalApproved); err != nil {
		return 0, 0, 0, fmt.Errorf("统计已通过提案失败: %w", err)
	}
	if rejected, err = count(domain.ProposalRejected); err != nil {
		return 0, 0, 0, fmt.Errorf("统计已驳回提案失败: %w", err)
	}
	return pending, approved, rejected, nil
}

// Approve 审核通过：把提案内容写入正式表 + 标记状态，**同一个事务**完成。
//
// apply 由 service 提供（按实体类型写入对应仓储），签名里带 tx 是为了让写入
// 复用同一个事务；payloadOverride 支持"作者修改后再通过"（规格书 §40）。
func (r *AnalysisProposalRepo) Approve(
	ctx context.Context,
	id string,
	note string,
	payloadOverride map[string]any,
	apply func(tx *gorm.DB, p domain.AnalysisProposal) (string, error),
) (*domain.AnalysisProposal, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, domain.ErrProposalNotFound
	}

	var updated domain.AnalysisProposal
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var m proposalModel
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&m, "id = ?", id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrProposalNotFound
			}
			return fmt.Errorf("查询提案失败: %w", err)
		}
		if m.Status != string(domain.ProposalPending) {
			return domain.ErrProposalAlreadyDecided
		}

		p, err := toDomainProposal(m)
		if err != nil {
			return err
		}
		if len(payloadOverride) > 0 {
			merged := map[string]any{}
			for k, v := range p.Payload {
				merged[k] = v
			}
			for k, v := range payloadOverride {
				merged[k] = v
			}
			p.Payload = merged
		}
		if err := p.Validate(); err != nil {
			return err
		}

		appliedID, err := apply(tx, p)
		if err != nil {
			return err
		}

		now := time.Now().UTC()
		res := tx.Model(&proposalModel{}).Where("id = ?", id).Updates(map[string]any{
			"status":      string(domain.ProposalApproved),
			"review_note": strings.TrimSpace(note),
			"reviewed_at": now,
			"applied_id":  appliedID,
			"updated_at":  now,
			"payload":     mustJSON(p.Payload),
		})
		if res.Error != nil {
			return fmt.Errorf("更新提案状态失败: %w", res.Error)
		}
		p.Status = domain.ProposalApproved
		p.ReviewNote = strings.TrimSpace(note)
		p.ReviewedAt = &now
		p.AppliedID = &appliedID
		p.UpdatedAt = now
		updated = p
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &updated, nil
}

// Reject 驳回提案。
func (r *AnalysisProposalRepo) Reject(ctx context.Context, id, note string) (*domain.AnalysisProposal, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, domain.ErrProposalNotFound
	}
	var updated domain.AnalysisProposal
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var m proposalModel
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&m, "id = ?", id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrProposalNotFound
			}
			return fmt.Errorf("查询提案失败: %w", err)
		}
		if m.Status != string(domain.ProposalPending) {
			return domain.ErrProposalAlreadyDecided
		}
		now := time.Now().UTC()
		if err := tx.Model(&proposalModel{}).Where("id = ?", id).Updates(map[string]any{
			"status":      string(domain.ProposalRejected),
			"review_note": strings.TrimSpace(note),
			"reviewed_at": now,
			"updated_at":  now,
		}).Error; err != nil {
			return fmt.Errorf("驳回提案失败: %w", err)
		}
		p, err := toDomainProposal(m)
		if err != nil {
			return err
		}
		p.Status = domain.ProposalRejected
		p.ReviewNote = strings.TrimSpace(note)
		p.ReviewedAt = &now
		p.UpdatedAt = now
		updated = p
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &updated, nil
}

func toDomainProposal(m proposalModel) (domain.AnalysisProposal, error) {
	p := domain.AnalysisProposal{
		ID: m.ID, WorkID: m.WorkID, TaskID: m.TaskID,
		Stage: domain.AnalysisStage(m.Stage), EntityType: domain.ProposalEntity(m.EntityType),
		Title: m.Title, Evidence: m.Evidence, Confidence: m.Confidence,
		Status: domain.ProposalStatus(m.Status), ReviewNote: m.ReviewNote,
		ReviewedAt: m.ReviewedAt, AppliedID: m.AppliedID,
		CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
		Payload: map[string]any{},
	}
	if m.Payload != "" {
		if err := json.Unmarshal([]byte(m.Payload), &p.Payload); err != nil {
			return domain.AnalysisProposal{}, fmt.Errorf("解析提案内容失败: %w", err)
		}
	}
	if m.DeletedAt.Valid {
		t := m.DeletedAt.Time
		p.DeletedAt = &t
	}
	return p, nil
}

func mustJSON(v map[string]any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(raw)
}
