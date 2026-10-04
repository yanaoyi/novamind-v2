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

type outlineModel struct {
	ID             string         `gorm:"column:id;type:uuid;primaryKey"`
	CreativeWorkID string         `gorm:"column:creative_work_id;type:uuid;not null"`
	Title          string         `gorm:"column:title;size:200;not null"`
	Summary        string         `gorm:"column:summary;not null;default:''"`
	Version        int            `gorm:"column:version;not null"`
	Source         string         `gorm:"column:source;size:10;not null"`
	CreatedAt      time.Time      `gorm:"column:created_at;not null"`
	UpdatedAt      time.Time      `gorm:"column:updated_at;not null"`
	DeletedAt      gorm.DeletedAt `gorm:"column:deleted_at;index"`
}

func (outlineModel) TableName() string { return "outlines" }

type outlineNodeModel struct {
	ID         string         `gorm:"column:id;type:uuid;primaryKey"`
	OutlineID  string         `gorm:"column:outline_id;type:uuid;not null"`
	ParentID   *string        `gorm:"column:parent_id;type:uuid"`
	Level      int16          `gorm:"column:level;not null"`
	Sequence   int            `gorm:"column:sequence;not null"`
	Title      string         `gorm:"column:title;size:200;not null"`
	Summary    string         `gorm:"column:summary;not null;default:''"`
	Purpose    string         `gorm:"column:purpose;not null;default:''"`
	Characters string         `gorm:"column:characters;type:jsonb;not null;default:'[]'"`
	Location   string         `gorm:"column:location;size:200;not null;default:''"`
	Conflict   string         `gorm:"column:conflict;not null;default:''"`
	Outcome    string         `gorm:"column:outcome;not null;default:''"`
	CreatedAt  time.Time      `gorm:"column:created_at;not null"`
	UpdatedAt  time.Time      `gorm:"column:updated_at;not null"`
	DeletedAt  gorm.DeletedAt `gorm:"column:deleted_at;index"`
}

func (outlineNodeModel) TableName() string { return "outline_nodes" }

// OutlineRepo 是大纲仓储（大纲 + 大纲节点树，规格书 §27）。
type OutlineRepo struct {
	db *gorm.DB
}

// NewOutlineRepo 构建仓储。
func NewOutlineRepo(db *gorm.DB) *OutlineRepo { return &OutlineRepo{db: db} }

// ---------- 大纲 ----------

// CreateOutline 新增大纲。
func (r *OutlineRepo) CreateOutline(ctx context.Context, o *domain.Outline) error {
	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("生成大纲 ID 失败: %w", err)
	}
	o.ID = id.String()
	now := time.Now().UTC()
	o.CreatedAt, o.UpdatedAt = now, now
	if err := r.db.WithContext(ctx).Create(&outlineModel{
		ID: o.ID, CreativeWorkID: o.CreativeWorkID, Title: o.Title, Summary: o.Summary,
		Version: o.Version, Source: string(o.Source), CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		if isForeignKeyViolation(err) {
			return domain.ErrCreativeNotFound
		}
		return fmt.Errorf("创建大纲失败: %w", err)
	}
	return nil
}

// ListOutlines 列出某个二创作品的大纲（带节点数）。
func (r *OutlineRepo) ListOutlines(ctx context.Context, workID string) ([]domain.Outline, error) {
	var models []outlineModel
	if err := r.db.WithContext(ctx).Where("creative_work_id = ?", workID).
		Order("created_at ASC").Find(&models).Error; err != nil {
		return nil, fmt.Errorf("查询大纲失败: %w", err)
	}
	out := make([]domain.Outline, 0, len(models))
	for _, m := range models {
		var count int64
		if err := r.db.WithContext(ctx).Model(&outlineNodeModel{}).
			Where("outline_id = ?", m.ID).Count(&count).Error; err != nil {
			return nil, fmt.Errorf("统计大纲节点失败: %w", err)
		}
		out = append(out, domain.Outline{
			ID: m.ID, CreativeWorkID: m.CreativeWorkID, Title: m.Title, Summary: m.Summary,
			Version: m.Version, Source: domain.OutlineSource(m.Source),
			CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt, NodeCount: int(count),
		})
	}
	return out, nil
}

// GetOutline 取大纲。
func (r *OutlineRepo) GetOutline(ctx context.Context, id string) (*domain.Outline, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, domain.ErrOutlineNotFound
	}
	var m outlineModel
	if err := r.db.WithContext(ctx).First(&m, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrOutlineNotFound
		}
		return nil, fmt.Errorf("查询大纲失败: %w", err)
	}
	return &domain.Outline{
		ID: m.ID, CreativeWorkID: m.CreativeWorkID, Title: m.Title, Summary: m.Summary,
		Version: m.Version, Source: domain.OutlineSource(m.Source),
		CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}, nil
}

// UpdateOutline 更新大纲元信息（标题 / 概要 / 版本号）。
func (r *OutlineRepo) UpdateOutline(ctx context.Context, o *domain.Outline) error {
	res := r.db.WithContext(ctx).Model(&outlineModel{}).Where("id = ?", o.ID).Updates(map[string]any{
		"title": o.Title, "summary": o.Summary, "version": o.Version,
		"source": string(o.Source), "updated_at": time.Now().UTC(),
	})
	if res.Error != nil {
		return fmt.Errorf("更新大纲失败: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.ErrOutlineNotFound
	}
	return nil
}

// DeleteOutline 软删大纲及其全部节点。
func (r *OutlineRepo) DeleteOutline(ctx context.Context, id string) error {
	now := time.Now().UTC()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&outlineNodeModel{}).
			Where("outline_id = ? AND deleted_at IS NULL", id).
			Updates(map[string]any{"deleted_at": now, "updated_at": now}).Error; err != nil {
			return fmt.Errorf("删除大纲节点失败: %w", err)
		}
		res := tx.Model(&outlineModel{}).Where("id = ?", id).
			Updates(map[string]any{"deleted_at": now, "updated_at": now})
		if res.Error != nil {
			return fmt.Errorf("删除大纲失败: %w", res.Error)
		}
		if res.RowsAffected == 0 {
			return domain.ErrOutlineNotFound
		}
		return nil
	})
}

// ---------- 节点 ----------

// ListNodes 列出某大纲的全部节点（扁平，供 service 组树）。
func (r *OutlineRepo) ListNodes(ctx context.Context, outlineID string) ([]domain.OutlineNode, error) {
	var models []outlineNodeModel
	if err := r.db.WithContext(ctx).Where("outline_id = ?", outlineID).
		Order("level ASC, sequence ASC").Find(&models).Error; err != nil {
		return nil, fmt.Errorf("查询大纲节点失败: %w", err)
	}
	out := make([]domain.OutlineNode, 0, len(models))
	for _, m := range models {
		out = append(out, toDomainOutlineNode(m))
	}
	return out, nil
}

// GetNode 取单个节点。
func (r *OutlineRepo) GetNode(ctx context.Context, id string) (*domain.OutlineNode, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, domain.ErrOutlineNodeNotFound
	}
	var m outlineNodeModel
	if err := r.db.WithContext(ctx).First(&m, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrOutlineNodeNotFound
		}
		return nil, fmt.Errorf("查询大纲节点失败: %w", err)
	}
	n := toDomainOutlineNode(m)
	return &n, nil
}

// CreateNode 新增单个节点。
func (r *OutlineRepo) CreateNode(ctx context.Context, n *domain.OutlineNode) error {
	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("生成大纲节点 ID 失败: %w", err)
	}
	n.ID = id.String()
	now := time.Now().UTC()
	n.CreatedAt, n.UpdatedAt = now, now
	characters, err := marshalCharacters(n.Characters)
	if err != nil {
		return err
	}
	if err := r.db.WithContext(ctx).Create(&outlineNodeModel{
		ID: n.ID, OutlineID: n.OutlineID, ParentID: n.ParentID, Level: int16(n.Level),
		Sequence: n.Sequence, Title: n.Title, Summary: n.Summary, Purpose: n.Purpose,
		Characters: characters, Location: n.Location, Conflict: n.Conflict, Outcome: n.Outcome,
		CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		if isForeignKeyViolation(err) {
			return domain.ErrOutlineParentNotFound
		}
		return fmt.Errorf("创建大纲节点失败: %w", err)
	}
	return nil
}

// UpdateNode 更新节点字段。
func (r *OutlineRepo) UpdateNode(ctx context.Context, n *domain.OutlineNode) error {
	characters, err := marshalCharacters(n.Characters)
	if err != nil {
		return err
	}
	res := r.db.WithContext(ctx).Model(&outlineNodeModel{}).Where("id = ?", n.ID).Updates(map[string]any{
		"title": n.Title, "summary": n.Summary, "purpose": n.Purpose,
		"characters": characters, "location": n.Location, "conflict": n.Conflict,
		"outcome": n.Outcome, "sequence": n.Sequence, "updated_at": time.Now().UTC(),
	})
	if res.Error != nil {
		return fmt.Errorf("更新大纲节点失败: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.ErrOutlineNodeNotFound
	}
	return nil
}

// DeleteNodeSubtree 软删节点及其所有后代，返回删除的节点数。
//
// 用递归 CTE 一次搞定：先删子节点再删父节点会漏掉"重挂子节点"的中间态，
// 而作者点删除时预期的是"这一支都没了"。
func (r *OutlineRepo) DeleteNodeSubtree(ctx context.Context, id string) (int, error) {
	if _, err := uuid.Parse(id); err != nil {
		return 0, domain.ErrOutlineNodeNotFound
	}
	var exists int64
	if err := r.db.WithContext(ctx).Model(&outlineNodeModel{}).Where("id = ?", id).Count(&exists).Error; err != nil {
		return 0, fmt.Errorf("查询大纲节点失败: %w", err)
	}
	if exists == 0 {
		return 0, domain.ErrOutlineNodeNotFound
	}
	now := time.Now().UTC()
	res := r.db.WithContext(ctx).Exec(`
		WITH RECURSIVE sub AS (
			SELECT id FROM outline_nodes WHERE id = ? AND deleted_at IS NULL
			UNION ALL
			SELECT n.id FROM outline_nodes n JOIN sub s ON n.parent_id = s.id WHERE n.deleted_at IS NULL
		)
		UPDATE outline_nodes SET deleted_at = ?, updated_at = ?
		WHERE id IN (SELECT id FROM sub) AND deleted_at IS NULL`, id, now, now)
	if res.Error != nil {
		return 0, fmt.Errorf("删除大纲节点失败: %w", res.Error)
	}
	return int(res.RowsAffected), nil
}

// NextNodeSequence 取某个父节点下的下一个序号。
func (r *OutlineRepo) NextNodeSequence(ctx context.Context, outlineID string, parentID *string) (int, error) {
	query := r.db.WithContext(ctx).Model(&outlineNodeModel{}).Where("outline_id = ?", outlineID)
	if parentID == nil {
		query = query.Where("parent_id IS NULL")
	} else {
		query = query.Where("parent_id = ?", *parentID)
	}
	var maxSeq int
	if err := query.Select("COALESCE(MAX(sequence), 0)").Scan(&maxSeq).Error; err != nil {
		return 0, fmt.Errorf("查询节点序号失败: %w", err)
	}
	return maxSeq + 1, nil
}

// CreateNodeLocked 在事务里取号并写入节点（同一份大纲串行化）。
//
// 审查 P2：原先是"先 MAX(sequence)+1 取号、再另开一条语句插入"，
// 两个请求并发时会拿到同一个序号，节点顺序出现歧义。
// 这里用大纲级 advisory lock 把同一份大纲的取号+插入串起来：
// 锁只在事务内有效，且只锁这一份大纲，不影响其它大纲的并发写入。
func (r *OutlineRepo) CreateNodeLocked(ctx context.Context, n *domain.OutlineNode) error {
	now := time.Now().UTC()
	characters, err := marshalCharacters(n.Characters)
	if err != nil {
		return err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("生成大纲节点 ID 失败: %w", err)
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?))", n.OutlineID).Error; err != nil {
			return fmt.Errorf("锁定大纲失败: %w", err)
		}
		if n.Sequence <= 0 {
			seq, err := nextNodeSequenceWith(ctx, tx, n.OutlineID, n.ParentID)
			if err != nil {
				return err
			}
			n.Sequence = seq
		}
		n.ID = id.String()
		n.CreatedAt, n.UpdatedAt = now, now
		if err := tx.Create(&outlineNodeModel{
			ID: n.ID, OutlineID: n.OutlineID, ParentID: n.ParentID, Level: int16(n.Level),
			Sequence: n.Sequence, Title: n.Title, Summary: n.Summary, Purpose: n.Purpose,
			Characters: characters, Location: n.Location, Conflict: n.Conflict, Outcome: n.Outcome,
			CreatedAt: now, UpdatedAt: now,
		}).Error; err != nil {
			if isForeignKeyViolation(err) {
				return domain.ErrOutlineParentNotFound
			}
			return fmt.Errorf("创建大纲节点失败: %w", err)
		}
		return nil
	})
}

// nextNodeSequenceWith 是 NextNodeSequence 的事务版实现（可传 tx）。
func nextNodeSequenceWith(ctx context.Context, db *gorm.DB, outlineID string, parentID *string) (int, error) {
	query := db.WithContext(ctx).Model(&outlineNodeModel{}).Where("outline_id = ?", outlineID)
	if parentID == nil {
		query = query.Where("parent_id IS NULL")
	} else {
		query = query.Where("parent_id = ?", *parentID)
	}
	var maxSeq int
	if err := query.Select("COALESCE(MAX(sequence), 0)").Scan(&maxSeq).Error; err != nil {
		return 0, fmt.Errorf("查询节点序号失败: %w", err)
	}
	return maxSeq + 1, nil
}

// ReplaceTree 用一整棵树替换大纲节点（事务内先软删旧节点，再按父子顺序插入新节点）。
//
// parents[i] 是新节点序列中第 i 个节点的父节点下标（-1 = 根）。
// 返回写入后的节点（含生成的 ID 与回填的 parent_id）。
func (r *OutlineRepo) ReplaceTree(
	ctx context.Context,
	outlineID string,
	nodes []domain.OutlineNode,
	parents []int,
) ([]domain.OutlineNode, error) {
	return r.replaceTreeWith(ctx, r.db, outlineID, nodes, parents)
}

func (r *OutlineRepo) replaceTreeWith(
	ctx context.Context,
	db *gorm.DB,
	outlineID string,
	nodes []domain.OutlineNode,
	parents []int,
) ([]domain.OutlineNode, error) {
	if len(nodes) != len(parents) {
		return nil, errors.New("节点与父子关系长度不一致")
	}
	now := time.Now().UTC()
	written := make([]domain.OutlineNode, len(nodes))

	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&outlineNodeModel{}).
			Where("outline_id = ? AND deleted_at IS NULL", outlineID).
			Updates(map[string]any{"deleted_at": now, "updated_at": now}).Error; err != nil {
			return fmt.Errorf("清空旧大纲节点失败: %w", err)
		}
		for i, n := range nodes {
			id, err := uuid.NewV7()
			if err != nil {
				return fmt.Errorf("生成大纲节点 ID 失败: %w", err)
			}
			n.ID = id.String()
			n.OutlineID = outlineID
			n.CreatedAt, n.UpdatedAt = now, now
			if parent := parents[i]; parent >= 0 {
				if parent >= i {
					return errors.New("父节点必须先于子节点写入")
				}
				pid := written[parent].ID
				n.ParentID = &pid
			} else {
				n.ParentID = nil
			}
			characters, err := marshalCharacters(n.Characters)
			if err != nil {
				return err
			}
			if err := tx.Create(&outlineNodeModel{
				ID: n.ID, OutlineID: outlineID, ParentID: n.ParentID, Level: int16(n.Level),
				Sequence: n.Sequence, Title: n.Title, Summary: n.Summary, Purpose: n.Purpose,
				Characters: characters, Location: n.Location, Conflict: n.Conflict, Outcome: n.Outcome,
				CreatedAt: now, UpdatedAt: now,
			}).Error; err != nil {
				return fmt.Errorf("写入大纲节点失败: %w", err)
			}
			written[i] = n
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return written, nil
}

// RestoreTreeWithMeta 在一次事务里完成"整树替换 + 大纲元信息（标题/概要/版本/来源）回填"。
//
// 审查 P1-7：版本恢复原本是 ReplaceTree → UpdateOutline → SnapshotTree 三步，
// 若 UpdateOutline 失败，就会留下"树已经是旧版、标题/版本号还是新版"的不一致状态。
// 恢复要么整体生效、要么整体不生效。
func (r *OutlineRepo) RestoreTreeWithMeta(
	ctx context.Context,
	outline *domain.Outline,
	nodes []domain.OutlineNode,
	parents []int,
) ([]domain.OutlineNode, error) {
	if len(nodes) != len(parents) {
		return nil, errors.New("节点与父子关系长度不一致")
	}
	var written []domain.OutlineNode
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		written, err = r.replaceTreeWith(ctx, tx, outline.ID, nodes, parents)
		if err != nil {
			return err
		}
		res := tx.Model(&outlineModel{}).Where("id = ?", outline.ID).Updates(map[string]any{
			"title": outline.Title, "summary": outline.Summary, "version": outline.Version,
			"source": string(outline.Source), "updated_at": time.Now().UTC(),
		})
		if res.Error != nil {
			return fmt.Errorf("回填大纲元信息失败: %w", res.Error)
		}
		if res.RowsAffected == 0 {
			return domain.ErrOutlineNotFound
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return written, nil
}

// ---------- 转换 ----------

func toDomainOutlineNode(m outlineNodeModel) domain.OutlineNode {
	characters := []string{}
	unmarshalJSONB("outline_nodes", "characters", m.Characters, &characters)
	return domain.OutlineNode{
		ID: m.ID, OutlineID: m.OutlineID, ParentID: m.ParentID, Level: domain.OutlineLevel(m.Level),
		Sequence: m.Sequence, Title: m.Title, Summary: m.Summary, Purpose: m.Purpose,
		Characters: characters, Location: m.Location, Conflict: m.Conflict, Outcome: m.Outcome,
		CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}
}

func marshalCharacters(characters []string) (string, error) {
	if characters == nil {
		characters = []string{}
	}
	raw, err := json.Marshal(characters)
	if err != nil {
		return "", fmt.Errorf("序列化人物列表失败: %w", err)
	}
	return string(raw), nil
}
