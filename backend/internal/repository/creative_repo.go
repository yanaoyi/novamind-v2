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

type creativeWorkModel struct {
	ID                string         `gorm:"column:id;type:uuid;primaryKey"`
	ProjectID         string         `gorm:"column:project_id;type:uuid;not null"`
	OriginalWorkID    string         `gorm:"column:original_work_id;type:uuid;not null"`
	Title             string         `gorm:"column:title;size:200;not null"`
	Description       string         `gorm:"column:description;not null;default:''"`
	Status            string         `gorm:"column:status;size:20;not null;default:DRAFT"`
	DivergencePointID *string        `gorm:"column:divergence_point_id;type:uuid"`
	CreatedAt         time.Time      `gorm:"column:created_at;not null"`
	UpdatedAt         time.Time      `gorm:"column:updated_at;not null"`
	DeletedAt         gorm.DeletedAt `gorm:"column:deleted_at;index"`
}

func (creativeWorkModel) TableName() string { return "creative_works" }

type creativeCharacterModel struct {
	ID                string         `gorm:"column:id;type:uuid;primaryKey"`
	CreativeWorkID    string         `gorm:"column:creative_work_id;type:uuid;not null"`
	Name              string         `gorm:"column:name;size:120;not null"`
	Description       string         `gorm:"column:description;not null;default:''"`
	SourceType        string         `gorm:"column:source_type;size:25;not null"`
	SourceCharacterID *string        `gorm:"column:source_character_id;type:uuid"`
	DNA               string         `gorm:"column:dna;type:jsonb;not null;default:'{}'"`
	FusionSources     string         `gorm:"column:fusion_sources;type:jsonb;not null;default:'[]'"`
	FusionDetail      string         `gorm:"column:fusion_detail;type:jsonb;not null;default:'[]'"`
	Modifications     string         `gorm:"column:modifications;type:jsonb;not null;default:'{}'"`
	Importance        int            `gorm:"column:importance;not null;default:3"`
	IsLocked          bool           `gorm:"column:is_locked;not null;default:false"`
	CreatedAt         time.Time      `gorm:"column:created_at;not null"`
	UpdatedAt         time.Time      `gorm:"column:updated_at;not null"`
	DeletedAt         gorm.DeletedAt `gorm:"column:deleted_at;index"`
}

func (creativeCharacterModel) TableName() string { return "creative_characters" }

type inheritanceRuleModel struct {
	ID                  string `gorm:"column:id;type:uuid;primaryKey"`
	CreativeCharacterID string `gorm:"column:creative_character_id;type:uuid;not null"`
	SourceCharacterID   string `gorm:"column:source_character_id;type:uuid;not null"`
	// 注意：这里不能写 gorm 的 default 标签。GORM 会把"带 default 的零值字段"从 INSERT 里省略、
	// 交给数据库默认值 —— 而继承权重里 0 是有意义的（表示"不继承该维度"），
	// 一旦被默认值 100 顶替，就变成"完全继承"，属于静默的数据错误（冒烟测试抓到过）。
	PersonalityWeight  int            `gorm:"column:personality_weight;not null"`
	ValueWeight        int            `gorm:"column:value_weight;not null"`
	MotivationWeight   int            `gorm:"column:motivation_weight;not null"`
	BehaviorWeight     int            `gorm:"column:behavior_weight;not null"`
	SpeechWeight       int            `gorm:"column:speech_weight;not null"`
	BackgroundWeight   int            `gorm:"column:background_weight;not null"`
	AbilityWeight      int            `gorm:"column:ability_weight;not null"`
	RelationshipWeight int            `gorm:"column:relationship_weight;not null"`
	CreatedAt          time.Time      `gorm:"column:created_at;not null"`
	UpdatedAt          time.Time      `gorm:"column:updated_at;not null"`
	DeletedAt          gorm.DeletedAt `gorm:"column:deleted_at;index"`
}

func (inheritanceRuleModel) TableName() string { return "inheritance_rules" }

type mappingModel struct {
	ID             string         `gorm:"column:id;type:uuid;primaryKey"`
	CreativeWorkID string         `gorm:"column:creative_work_id;type:uuid;not null"`
	OriginalType   string         `gorm:"column:original_type;size:30;not null"`
	OriginalID     string         `gorm:"column:original_id;type:uuid;not null"`
	CreativeType   string         `gorm:"column:creative_type;size:30;not null"`
	CreativeID     string         `gorm:"column:creative_id;type:uuid;not null"`
	MappingType    string         `gorm:"column:mapping_type;size:20;not null"`
	Description    string         `gorm:"column:description;not null;default:''"`
	CreatedAt      time.Time      `gorm:"column:created_at;not null"`
	UpdatedAt      time.Time      `gorm:"column:updated_at;not null"`
	DeletedAt      gorm.DeletedAt `gorm:"column:deleted_at;index"`
}

func (mappingModel) TableName() string { return "original_creative_mappings" }

// CreativeRepo 是二创核心仓储。
type CreativeRepo struct {
	db *gorm.DB
}

// NewCreativeRepo 构建仓储。
func NewCreativeRepo(db *gorm.DB) *CreativeRepo { return &CreativeRepo{db: db} }

// ---------- 二创作品 ----------

// CreateWork 创建二创作品。
func (r *CreativeRepo) CreateWork(ctx context.Context, w *domain.CreativeWork) error {
	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("生成二创作品 ID 失败: %w", err)
	}
	w.ID = id.String()
	now := time.Now().UTC()
	w.CreatedAt, w.UpdatedAt = now, now

	m := creativeWorkModel{
		ID: w.ID, ProjectID: w.ProjectID, OriginalWorkID: w.OriginalWorkID,
		Title: w.Title, Description: w.Description, Status: string(w.Status),
		CreatedAt: w.CreatedAt, UpdatedAt: w.UpdatedAt,
	}
	if err := r.db.WithContext(ctx).Create(&m).Error; err != nil {
		if isUniqueViolation(err) {
			return domain.ErrCreativeAlreadyExists
		}
		if isForeignKeyViolation(err) {
			return errors.New("文章或原著不存在")
		}
		return fmt.Errorf("创建二创作品失败: %w", err)
	}
	return nil
}

// GetWorkByID 取二创作品。
func (r *CreativeRepo) GetWorkByID(ctx context.Context, id string) (*domain.CreativeWork, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, domain.ErrCreativeNotFound
	}
	var m creativeWorkModel
	if err := r.db.WithContext(ctx).First(&m, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrCreativeNotFound
		}
		return nil, fmt.Errorf("查询二创作品失败: %w", err)
	}
	w := toDomainCreativeWork(m)
	return &w, nil
}

// GetWorkByProject 按文章取二创作品。
func (r *CreativeRepo) GetWorkByProject(ctx context.Context, projectID string) (*domain.CreativeWork, error) {
	if _, err := uuid.Parse(projectID); err != nil {
		return nil, domain.ErrCreativeNotFound
	}
	var m creativeWorkModel
	if err := r.db.WithContext(ctx).First(&m, "project_id = ?", projectID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrCreativeNotFound
		}
		return nil, fmt.Errorf("查询二创作品失败: %w", err)
	}
	w := toDomainCreativeWork(m)
	return &w, nil
}

// ListWorksByOriginal 列出某原著下的二创作品。
func (r *CreativeRepo) ListWorksByOriginal(ctx context.Context, originalWorkID string) ([]domain.CreativeWork, error) {
	query := r.db.WithContext(ctx).Model(&creativeWorkModel{})
	if originalWorkID != "" {
		query = query.Where("original_work_id = ?", originalWorkID)
	}
	var models []creativeWorkModel
	if err := query.Order("created_at DESC").Find(&models).Error; err != nil {
		return nil, fmt.Errorf("查询二创作品失败: %w", err)
	}
	out := make([]domain.CreativeWork, 0, len(models))
	for _, m := range models {
		out = append(out, toDomainCreativeWork(m))
	}
	return out, nil
}

// UpdateWork 更新二创作品（标题/简介/状态）。
func (r *CreativeRepo) UpdateWork(ctx context.Context, w *domain.CreativeWork) error {
	now := time.Now().UTC()
	res := r.db.WithContext(ctx).Model(&creativeWorkModel{}).Where("id = ?", w.ID).Updates(map[string]any{
		"title": w.Title, "description": w.Description, "status": string(w.Status), "updated_at": now,
	})
	if res.Error != nil {
		return fmt.Errorf("更新二创作品失败: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		if _, err := r.GetWorkByID(ctx, w.ID); err != nil {
			return err
		}
	}
	w.UpdatedAt = now
	return nil
}

// SetDivergencePoint 记录分叉点 ID。
func (r *CreativeRepo) SetDivergencePoint(ctx context.Context, workID, divergencePointID string) error {
	res := r.db.WithContext(ctx).Model(&creativeWorkModel{}).Where("id = ?", workID).Updates(map[string]any{
		"divergence_point_id": divergencePointID, "updated_at": time.Now().UTC(),
	})
	if res.Error != nil {
		return fmt.Errorf("记录分叉点失败: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.ErrCreativeNotFound
	}
	return nil
}

// ---------- 二创人物 ----------

// CreateCharacter 创建二创人物（DNA/融合说明一起写入）。
func (r *CreativeRepo) CreateCharacter(ctx context.Context, c *domain.CreativeCharacter) error {
	return r.createCharacterWith(ctx, r.db, c)
}

// createCharacterWith 是 CreateCharacter 的事务版实现（可传 tx）。
func (r *CreativeRepo) createCharacterWith(ctx context.Context, db *gorm.DB, c *domain.CreativeCharacter) error {
	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("生成二创人物 ID 失败: %w", err)
	}
	c.ID = id.String()
	now := time.Now().UTC()
	c.CreatedAt, c.UpdatedAt = now, now

	m, err := toCreativeCharacterModel(c)
	if err != nil {
		return err
	}
	if err := db.WithContext(ctx).Create(&m).Error; err != nil {
		if isUniqueViolation(err) {
			return domain.ErrCreativeCharacterDup
		}
		if isForeignKeyViolation(err) {
			return errors.New("二创作品或原著人物不存在")
		}
		return fmt.Errorf("创建二创人物失败: %w", err)
	}
	return nil
}

// GetCharacter 取二创人物。
func (r *CreativeRepo) GetCharacter(ctx context.Context, id string) (*domain.CreativeCharacter, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, domain.ErrCreativeCharacterNotFound
	}
	var m creativeCharacterModel
	if err := r.db.WithContext(ctx).First(&m, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrCreativeCharacterNotFound
		}
		return nil, fmt.Errorf("查询二创人物失败: %w", err)
	}
	c, err := toDomainCreativeCharacter(m)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// ListCharacters 列出二创作品下的人物。
func (r *CreativeRepo) ListCharacters(ctx context.Context, workID string) ([]domain.CreativeCharacter, error) {
	if _, err := uuid.Parse(workID); err != nil {
		return nil, domain.ErrCreativeNotFound
	}
	var models []creativeCharacterModel
	if err := r.db.WithContext(ctx).Where("creative_work_id = ?", workID).
		Order("importance DESC, created_at ASC").Find(&models).Error; err != nil {
		return nil, fmt.Errorf("查询二创人物失败: %w", err)
	}
	out := make([]domain.CreativeCharacter, 0, len(models))
	for _, m := range models {
		c, err := toDomainCreativeCharacter(m)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

// UpdateCharacter 更新二创人物；锁定的角色拒绝覆盖继承结果。
func (r *CreativeRepo) UpdateCharacter(ctx context.Context, c *domain.CreativeCharacter) error {
	existing, err := r.GetCharacter(ctx, c.ID)
	if err != nil {
		return err
	}
	if existing.IsLocked && existing.SourceType != c.SourceType {
		return domain.ErrCreativeCharacterLocked
	}
	return r.updateCharacterWith(ctx, r.db, c)
}

// updateCharacterWith 是 UpdateCharacter 的事务版实现（不含锁定校验，调用方负责）。
func (r *CreativeRepo) updateCharacterWith(ctx context.Context, db *gorm.DB, c *domain.CreativeCharacter) error {
	m, err := toCreativeCharacterModel(c)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	res := db.WithContext(ctx).Model(&creativeCharacterModel{}).Where("id = ?", c.ID).Updates(map[string]any{
		"name":           m.Name,
		"description":    m.Description,
		"source_type":    m.SourceType,
		"dna":            m.DNA,
		"fusion_sources": m.FusionSources,
		"fusion_detail":  m.FusionDetail,
		"modifications":  m.Modifications,
		"importance":     m.Importance,
		"is_locked":      m.IsLocked,
		"updated_at":     now,
	})
	if res.Error != nil {
		if isUniqueViolation(res.Error) {
			return domain.ErrCreativeCharacterDup
		}
		return fmt.Errorf("更新二创人物失败: %w", res.Error)
	}
	c.UpdatedAt = now
	return nil
}

// DeleteCharacter 软删除二创人物。
func (r *CreativeRepo) DeleteCharacter(ctx context.Context, id string) error {
	if _, err := uuid.Parse(id); err != nil {
		return domain.ErrCreativeCharacterNotFound
	}
	res := r.db.WithContext(ctx).Where("id = ?", id).Delete(&creativeCharacterModel{})
	if res.Error != nil {
		return fmt.Errorf("删除二创人物失败: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.ErrCreativeCharacterNotFound
	}
	return nil
}

// ---------- 继承规则 ----------

// UpsertInheritanceRule 写入/更新继承权重。
func (r *CreativeRepo) UpsertInheritanceRule(ctx context.Context, rule *domain.InheritanceRule) error {
	return r.upsertInheritanceRuleWith(ctx, r.db, rule)
}

// upsertInheritanceRuleWith 是 UpsertInheritanceRule 的事务版实现（可传 tx）。
func (r *CreativeRepo) upsertInheritanceRuleWith(ctx context.Context, db *gorm.DB, rule *domain.InheritanceRule) error {
	var existing inheritanceRuleModel
	err := db.WithContext(ctx).
		First(&existing, "creative_character_id = ? AND source_character_id = ?", rule.CreativeCharacterID, rule.SourceCharacterID).Error
	now := time.Now().UTC()

	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		id, genErr := uuid.NewV7()
		if genErr != nil {
			return fmt.Errorf("生成继承规则 ID 失败: %w", genErr)
		}
		rule.ID = id.String()
		rule.CreatedAt, rule.UpdatedAt = now, now
		m := toInheritanceModel(*rule)
		if err := db.WithContext(ctx).Create(&m).Error; err != nil {
			if isForeignKeyViolation(err) {
				return domain.ErrSourceCharacterNotFound
			}
			return fmt.Errorf("写入继承规则失败: %w", err)
		}
		return nil
	case err != nil:
		return fmt.Errorf("查询继承规则失败: %w", err)
	}

	rule.ID = existing.ID
	rule.CreatedAt = existing.CreatedAt
	res := db.WithContext(ctx).Model(&inheritanceRuleModel{}).Where("id = ?", existing.ID).Updates(map[string]any{
		"personality_weight":  rule.PersonalityWeight,
		"value_weight":        rule.ValueWeight,
		"motivation_weight":   rule.MotivationWeight,
		"behavior_weight":     rule.BehaviorWeight,
		"speech_weight":       rule.SpeechWeight,
		"background_weight":   rule.BackgroundWeight,
		"ability_weight":      rule.AbilityWeight,
		"relationship_weight": rule.RelationshipWeight,
		"updated_at":          now,
	})
	if res.Error != nil {
		return fmt.Errorf("更新继承规则失败: %w", res.Error)
	}
	rule.UpdatedAt = now
	return nil
}

// GetInheritanceRule 取某二创人物对某原著人物的继承权重。
func (r *CreativeRepo) GetInheritanceRule(ctx context.Context, creativeCharacterID, sourceCharacterID string) (*domain.InheritanceRule, error) {
	var m inheritanceRuleModel
	if err := r.db.WithContext(ctx).
		First(&m, "creative_character_id = ? AND source_character_id = ?", creativeCharacterID, sourceCharacterID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("查询继承规则失败: %w", err)
	}
	rule := toDomainInheritance(m)
	return &rule, nil
}

// ListInheritanceRules 列出某二创人物的全部继承规则（融合人物会有多条）。
func (r *CreativeRepo) ListInheritanceRules(ctx context.Context, creativeCharacterID string) ([]domain.InheritanceRule, error) {
	if _, err := uuid.Parse(creativeCharacterID); err != nil {
		return nil, domain.ErrCreativeCharacterNotFound
	}
	var models []inheritanceRuleModel
	if err := r.db.WithContext(ctx).
		Where("creative_character_id = ?", creativeCharacterID).
		Order("created_at ASC").Find(&models).Error; err != nil {
		return nil, fmt.Errorf("查询继承规则失败: %w", err)
	}
	out := make([]domain.InheritanceRule, 0, len(models))
	for _, m := range models {
		out = append(out, toDomainInheritance(m))
	}
	return out, nil
}

// ---------- 映射 ----------

// CreateMapping 记录一条原著↔二创映射。
func (r *CreativeRepo) CreateMapping(ctx context.Context, m *domain.OriginalCreativeMapping) error {
	return r.upsertMappingWith(ctx, r.db, m)
}

// upsertMappingWith 写入映射：同一 (作品, 原著类型, 原著 ID, 二创 ID) 只保留一行。
//
// 为什么是 upsert 而不是 insert：重复继承同一个人物、或重新融合一次，
// 语义上是"同一个来源关系被更新"，不该在映射表里堆一串一模一样的行（审查 P1-3）。
// 冲突目标是 0017 迁移建的部分唯一索引（deleted_at IS NULL），所以这里要带上同样的谓词。
func (r *CreativeRepo) upsertMappingWith(ctx context.Context, db *gorm.DB, m *domain.OriginalCreativeMapping) error {
	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("生成映射 ID 失败: %w", err)
	}
	m.ID = id.String()
	now := time.Now().UTC()
	m.CreatedAt, m.UpdatedAt = now, now

	const upsertSQL = `
		INSERT INTO original_creative_mappings
			(id, creative_work_id, original_type, original_id, creative_type, creative_id,
			 mapping_type, description, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (creative_work_id, original_type, original_id, creative_id) WHERE deleted_at IS NULL
		DO UPDATE SET mapping_type = EXCLUDED.mapping_type,
		              description  = EXCLUDED.description,
		              updated_at   = EXCLUDED.updated_at
		RETURNING id`
	var storedID string
	res := db.WithContext(ctx).Raw(upsertSQL,
		m.ID, m.CreativeWorkID, m.OriginalType, m.OriginalID, m.CreativeType, m.CreativeID,
		string(m.MappingType), m.Description, now, now,
	).Scan(&storedID)
	if res.Error != nil {
		err := res.Error
		if isForeignKeyViolation(err) {
			return domain.ErrCreativeNotFound
		}
		return fmt.Errorf("写入映射失败: %w", err)
	}
	if storedID != "" {
		m.ID = storedID // 命中既有行时返回的是原本那行的 ID
	}
	return nil
}

// SaveInheritance 在一次事务里完成"人物（新建或更新）+ 继承权重 + 映射"三步写入。
//
// 审查 P1-4：这三步原本各自调用仓储方法，中间失败会留下"人物建了、权重没写"这类半成品，
// 而同一个项目的提案审核路径（applyProposal）本来就是走事务的 —— 实现不一致，这里补齐。
func (r *CreativeRepo) SaveInheritance(
	ctx context.Context,
	character *domain.CreativeCharacter,
	isNew bool,
	rule *domain.InheritanceRule,
	mapping *domain.OriginalCreativeMapping,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		if isNew {
			err = r.createCharacterWith(ctx, tx, character)
		} else {
			err = r.updateCharacterWith(ctx, tx, character)
		}
		if err != nil {
			return err
		}
		rule.CreativeCharacterID = character.ID
		if err := r.upsertInheritanceRuleWith(ctx, tx, rule); err != nil {
			return err
		}
		if mapping != nil {
			mapping.CreativeID = character.ID
			if err := r.upsertMappingWith(ctx, tx, mapping); err != nil {
				return err
			}
		}
		return nil
	})
}

// SaveFusion 在一次事务里完成"融合人物 + 多条来源映射"的写入。
func (r *CreativeRepo) SaveFusion(
	ctx context.Context,
	fused *domain.CreativeCharacter,
	mappings []*domain.OriginalCreativeMapping,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := r.createCharacterWith(ctx, tx, fused); err != nil {
			return err
		}
		for _, m := range mappings {
			m.CreativeID = fused.ID
			if err := r.upsertMappingWith(ctx, tx, m); err != nil {
				return err
			}
		}
		return nil
	})
}

// ListMappings 列出二创作品的全部映射。
func (r *CreativeRepo) ListMappings(ctx context.Context, workID string) ([]domain.OriginalCreativeMapping, error) {
	if _, err := uuid.Parse(workID); err != nil {
		return nil, domain.ErrCreativeNotFound
	}
	var models []mappingModel
	if err := r.db.WithContext(ctx).Where("creative_work_id = ?", workID).
		Order("created_at ASC").Find(&models).Error; err != nil {
		return nil, fmt.Errorf("查询映射失败: %w", err)
	}
	out := make([]domain.OriginalCreativeMapping, 0, len(models))
	for _, m := range models {
		out = append(out, toDomainMapping(m))
	}
	return out, nil
}

// DeleteMapping 软删除映射。
func (r *CreativeRepo) DeleteMapping(ctx context.Context, id string) error {
	if _, err := uuid.Parse(id); err != nil {
		return domain.ErrMappingNotFound
	}
	res := r.db.WithContext(ctx).Where("id = ?", id).Delete(&mappingModel{})
	if res.Error != nil {
		return fmt.Errorf("删除映射失败: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.ErrMappingNotFound
	}
	return nil
}

// ---------- 转换 ----------

// ---------- 二创世界 / 规则 / 分叉点 / 时间线 ----------

type creativeWorldModel struct {
	ID              string         `gorm:"column:id;type:uuid;primaryKey"`
	CreativeWorkID  string         `gorm:"column:creative_work_id;type:uuid;not null"`
	SourceWorldID   *string        `gorm:"column:source_world_id;type:uuid"`
	InheritanceMode string         `gorm:"column:inheritance_mode;size:10;not null"`
	Name            string         `gorm:"column:name;size:200;not null;default:''"`
	Description     string         `gorm:"column:description;not null;default:''"`
	CreatedAt       time.Time      `gorm:"column:created_at;not null"`
	UpdatedAt       time.Time      `gorm:"column:updated_at;not null"`
	DeletedAt       gorm.DeletedAt `gorm:"column:deleted_at;index"`
}

func (creativeWorldModel) TableName() string { return "creative_worlds" }

type creativeWorldRuleModel struct {
	ID              string         `gorm:"column:id;type:uuid;primaryKey"`
	CreativeWorldID string         `gorm:"column:creative_world_id;type:uuid;not null"`
	SourceRuleID    *string        `gorm:"column:source_rule_id;type:uuid"`
	Status          string         `gorm:"column:status;size:12;not null"`
	Category        string         `gorm:"column:category;size:60;not null;default:''"`
	Name            string         `gorm:"column:name;size:200;not null"`
	Description     string         `gorm:"column:description;not null;default:''"`
	Importance      int            `gorm:"column:importance;not null"`
	CreatedAt       time.Time      `gorm:"column:created_at;not null"`
	UpdatedAt       time.Time      `gorm:"column:updated_at;not null"`
	DeletedAt       gorm.DeletedAt `gorm:"column:deleted_at;index"`
}

func (creativeWorldRuleModel) TableName() string { return "creative_world_rules" }

type divergenceModel struct {
	ID                string         `gorm:"column:id;type:uuid;primaryKey"`
	CreativeWorkID    string         `gorm:"column:creative_work_id;type:uuid;not null"`
	OriginalChapterID *string        `gorm:"column:original_chapter_id;type:uuid"`
	OriginalEventID   *string        `gorm:"column:original_event_id;type:uuid"`
	TimeLabel         string         `gorm:"column:time_label;size:120;not null;default:''"`
	Description       string         `gorm:"column:description;not null;default:''"`
	CreatedAt         time.Time      `gorm:"column:created_at;not null"`
	UpdatedAt         time.Time      `gorm:"column:updated_at;not null"`
	DeletedAt         gorm.DeletedAt `gorm:"column:deleted_at;index"`
}

func (divergenceModel) TableName() string { return "divergence_points" }

type creativeTimelineModel struct {
	ID                    string         `gorm:"column:id;type:uuid;primaryKey"`
	CreativeWorkID        string         `gorm:"column:creative_work_id;type:uuid;not null"`
	SourceOriginalEventID *string        `gorm:"column:source_original_event_id;type:uuid"`
	Status                string         `gorm:"column:status;size:12;not null"`
	Sequence              int            `gorm:"column:sequence;not null"`
	TimeLabel             string         `gorm:"column:time_label;size:120;not null;default:''"`
	Title                 string         `gorm:"column:title;size:200;not null"`
	Description           string         `gorm:"column:description;not null;default:''"`
	CreatedAt             time.Time      `gorm:"column:created_at;not null"`
	UpdatedAt             time.Time      `gorm:"column:updated_at;not null"`
	DeletedAt             gorm.DeletedAt `gorm:"column:deleted_at;index"`
}

func (creativeTimelineModel) TableName() string { return "creative_timeline_events" }

// UpsertWorld 创建/更新二创世界（一个二创作品一个世界）。
func (r *CreativeRepo) UpsertWorld(ctx context.Context, w *domain.CreativeWorld) error {
	if _, err := uuid.Parse(w.CreativeWorkID); err != nil {
		return domain.ErrCreativeNotFound
	}
	var existing creativeWorldModel
	err := r.db.WithContext(ctx).First(&existing, "creative_work_id = ?", w.CreativeWorkID).Error
	now := time.Now().UTC()

	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		id, genErr := uuid.NewV7()
		if genErr != nil {
			return fmt.Errorf("生成二创世界 ID 失败: %w", genErr)
		}
		w.ID = id.String()
		w.CreatedAt, w.UpdatedAt = now, now
		m := creativeWorldModel{
			ID: w.ID, CreativeWorkID: w.CreativeWorkID, SourceWorldID: w.SourceWorldID,
			InheritanceMode: string(w.InheritanceMode), Name: w.Name, Description: w.Description,
			CreatedAt: w.CreatedAt, UpdatedAt: w.UpdatedAt,
		}
		if err := r.db.WithContext(ctx).Create(&m).Error; err != nil {
			if isForeignKeyViolation(err) {
				return domain.ErrCreativeNotFound
			}
			return fmt.Errorf("创建二创世界失败: %w", err)
		}
		return nil
	case err != nil:
		return fmt.Errorf("查询二创世界失败: %w", err)
	}

	updates := map[string]any{
		"inheritance_mode": string(w.InheritanceMode),
		"name":             w.Name,
		"description":      w.Description,
		"updated_at":       now,
	}
	if w.SourceWorldID != nil {
		updates["source_world_id"] = *w.SourceWorldID
	}
	if err := r.db.WithContext(ctx).Model(&creativeWorldModel{}).Where("id = ?", existing.ID).Updates(updates).Error; err != nil {
		return fmt.Errorf("更新二创世界失败: %w", err)
	}
	w.ID = existing.ID
	w.CreatedAt = existing.CreatedAt
	w.UpdatedAt = now
	return nil
}

// GetWorld 按二创作品取世界。
func (r *CreativeRepo) GetWorld(ctx context.Context, creativeWorkID string) (*domain.CreativeWorld, error) {
	if _, err := uuid.Parse(creativeWorkID); err != nil {
		return nil, domain.ErrCreativeWorldNotFound
	}
	var m creativeWorldModel
	if err := r.db.WithContext(ctx).First(&m, "creative_work_id = ?", creativeWorkID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrCreativeWorldNotFound
		}
		return nil, fmt.Errorf("查询二创世界失败: %w", err)
	}
	w := domain.CreativeWorld{
		ID: m.ID, CreativeWorkID: m.CreativeWorkID, SourceWorldID: m.SourceWorldID,
		InheritanceMode: domain.WorldInheritanceMode(m.InheritanceMode),
		Name:            m.Name, Description: m.Description,
		CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}
	return &w, nil
}

// GetWorkIDByCreativeWorld 由二创世界反查它所属的二创作品。
func (r *CreativeRepo) GetWorkIDByCreativeWorld(ctx context.Context, creativeWorldID string) (string, error) {
	if _, err := uuid.Parse(creativeWorldID); err != nil {
		return "", domain.ErrCreativeWorldNotFound
	}
	var m creativeWorldModel
	if err := r.db.WithContext(ctx).Select("id, creative_work_id").
		First(&m, "id = ?", creativeWorldID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", domain.ErrCreativeWorldNotFound
		}
		return "", fmt.Errorf("反查二创作品失败: %w", err)
	}
	return m.CreativeWorkID, nil
}

// CreateWorldRule 新增二创世界规则。
func (r *CreativeRepo) CreateWorldRule(ctx context.Context, rule *domain.CreativeWorldRule) error {
	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("生成二创规则 ID 失败: %w", err)
	}
	rule.ID = id.String()
	now := time.Now().UTC()
	rule.CreatedAt, rule.UpdatedAt = now, now
	m := creativeWorldRuleModel{
		ID: rule.ID, CreativeWorldID: rule.CreativeWorldID, SourceRuleID: rule.SourceRuleID,
		Status: string(rule.Status), Category: rule.Category, Name: rule.Name,
		Description: rule.Description, Importance: rule.Importance,
		CreatedAt: rule.CreatedAt, UpdatedAt: rule.UpdatedAt,
	}
	if err := r.db.WithContext(ctx).Create(&m).Error; err != nil {
		if isUniqueViolation(err) {
			return domain.ErrCreativeWorldRuleDup
		}
		if isForeignKeyViolation(err) {
			return domain.ErrCreativeWorldNotFound
		}
		return fmt.Errorf("创建二创规则失败: %w", err)
	}
	return nil
}

// ListWorldRules 列出二创世界规则。
func (r *CreativeRepo) ListWorldRules(ctx context.Context, creativeWorldID string) ([]domain.CreativeWorldRule, error) {
	var models []creativeWorldRuleModel
	if err := r.db.WithContext(ctx).Where("creative_world_id = ?", creativeWorldID).
		Order("importance DESC, created_at ASC").Find(&models).Error; err != nil {
		return nil, fmt.Errorf("查询二创规则失败: %w", err)
	}
	out := make([]domain.CreativeWorldRule, 0, len(models))
	for _, m := range models {
		out = append(out, toDomainCreativeWorldRule(m))
	}
	return out, nil
}

// GetWorldRule 取二创规则。
func (r *CreativeRepo) GetWorldRule(ctx context.Context, id string) (*domain.CreativeWorldRule, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, domain.ErrCreativeWorldRuleNotFound
	}
	var m creativeWorldRuleModel
	if err := r.db.WithContext(ctx).First(&m, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrCreativeWorldRuleNotFound
		}
		return nil, fmt.Errorf("查询二创规则失败: %w", err)
	}
	rule := toDomainCreativeWorldRule(m)
	return &rule, nil
}

// UpdateWorldRule 更新二创规则。
func (r *CreativeRepo) UpdateWorldRule(ctx context.Context, rule *domain.CreativeWorldRule) error {
	now := time.Now().UTC()
	res := r.db.WithContext(ctx).Model(&creativeWorldRuleModel{}).Where("id = ?", rule.ID).Updates(map[string]any{
		"status": string(rule.Status), "category": rule.Category, "name": rule.Name,
		"description": rule.Description, "importance": rule.Importance, "updated_at": now,
	})
	if res.Error != nil {
		if isUniqueViolation(res.Error) {
			return domain.ErrCreativeWorldRuleDup
		}
		return fmt.Errorf("更新二创规则失败: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		if _, err := r.GetWorldRule(ctx, rule.ID); err != nil {
			return err
		}
	}
	rule.UpdatedAt = now
	return nil
}

// DeleteWorldRule 软删除二创规则。
func (r *CreativeRepo) DeleteWorldRule(ctx context.Context, id string) error {
	if _, err := uuid.Parse(id); err != nil {
		return domain.ErrCreativeWorldRuleNotFound
	}
	res := r.db.WithContext(ctx).Where("id = ?", id).Delete(&creativeWorldRuleModel{})
	if res.Error != nil {
		return fmt.Errorf("删除二创规则失败: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.ErrCreativeWorldRuleNotFound
	}
	return nil
}

// UpsertDivergence 创建/更新分叉点。
func (r *CreativeRepo) UpsertDivergence(ctx context.Context, d *domain.DivergencePoint) error {
	if _, err := uuid.Parse(d.CreativeWorkID); err != nil {
		return domain.ErrCreativeNotFound
	}
	var existing divergenceModel
	err := r.db.WithContext(ctx).First(&existing, "creative_work_id = ?", d.CreativeWorkID).Error
	now := time.Now().UTC()

	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		id, genErr := uuid.NewV7()
		if genErr != nil {
			return fmt.Errorf("生成分叉点 ID 失败: %w", genErr)
		}
		d.ID = id.String()
		d.CreatedAt, d.UpdatedAt = now, now
		m := divergenceModel{
			ID: d.ID, CreativeWorkID: d.CreativeWorkID,
			OriginalChapterID: d.OriginalChapterID, OriginalEventID: d.OriginalEventID,
			TimeLabel: d.TimeLabel, Description: d.Description,
			CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt,
		}
		if err := r.db.WithContext(ctx).Create(&m).Error; err != nil {
			if isForeignKeyViolation(err) {
				return domain.ErrDivergenceSourceInvalid
			}
			return fmt.Errorf("创建分叉点失败: %w", err)
		}
		return nil
	case err != nil:
		return fmt.Errorf("查询分叉点失败: %w", err)
	}

	if err := r.db.WithContext(ctx).Model(&divergenceModel{}).Where("id = ?", existing.ID).Updates(map[string]any{
		"original_chapter_id": d.OriginalChapterID, "original_event_id": d.OriginalEventID,
		"time_label": d.TimeLabel, "description": d.Description, "updated_at": now,
	}).Error; err != nil {
		return fmt.Errorf("更新分叉点失败: %w", err)
	}
	d.ID = existing.ID
	d.CreatedAt = existing.CreatedAt
	d.UpdatedAt = now
	return nil
}

// GetDivergence 取分叉点。
func (r *CreativeRepo) GetDivergence(ctx context.Context, creativeWorkID string) (*domain.DivergencePoint, error) {
	if _, err := uuid.Parse(creativeWorkID); err != nil {
		return nil, domain.ErrDivergenceNotFound
	}
	var m divergenceModel
	if err := r.db.WithContext(ctx).First(&m, "creative_work_id = ?", creativeWorkID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrDivergenceNotFound
		}
		return nil, fmt.Errorf("查询分叉点失败: %w", err)
	}
	d := domain.DivergencePoint{
		ID: m.ID, CreativeWorkID: m.CreativeWorkID,
		OriginalChapterID: m.OriginalChapterID, OriginalEventID: m.OriginalEventID,
		TimeLabel: m.TimeLabel, Description: m.Description,
		CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}
	return &d, nil
}

// ReplaceTimeline 用给定顺序整体替换二创时间线（单事务，幂等）。
func (r *CreativeRepo) ReplaceTimeline(ctx context.Context, creativeWorkID string, events []domain.CreativeTimelineEvent) error {
	if _, err := uuid.Parse(creativeWorkID); err != nil {
		return domain.ErrCreativeNotFound
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("creative_work_id = ?", creativeWorkID).Delete(&creativeTimelineModel{}).Error; err != nil {
			return fmt.Errorf("清理旧二创时间线失败: %w", err)
		}
		now := time.Now().UTC()
		models := make([]creativeTimelineModel, 0, len(events))
		for i, e := range events {
			id, err := uuid.NewV7()
			if err != nil {
				return fmt.Errorf("生成二创时间线 ID 失败: %w", err)
			}
			models = append(models, creativeTimelineModel{
				ID: id.String(), CreativeWorkID: creativeWorkID, SourceOriginalEventID: e.SourceOriginalEventID,
				Status: string(e.Status), Sequence: i + 1, TimeLabel: e.TimeLabel,
				Title: e.Title, Description: e.Description, CreatedAt: now, UpdatedAt: now,
			})
		}
		if len(models) > 0 {
			if err := tx.CreateInBatches(&models, 200).Error; err != nil {
				if isUniqueViolation(err) {
					return fmt.Errorf("%w：同一条原著事件被重复继承", domain.ErrCreativeTimelineBad)
				}
				return fmt.Errorf("写入二创时间线失败: %w", err)
			}
		}
		return nil
	})
}

// ListTimeline 列出二创时间线。
func (r *CreativeRepo) ListTimeline(ctx context.Context, creativeWorkID string) ([]domain.CreativeTimelineEvent, error) {
	if _, err := uuid.Parse(creativeWorkID); err != nil {
		return nil, domain.ErrCreativeNotFound
	}
	var models []creativeTimelineModel
	if err := r.db.WithContext(ctx).Where("creative_work_id = ?", creativeWorkID).
		Order("sequence ASC").Find(&models).Error; err != nil {
		return nil, fmt.Errorf("查询二创时间线失败: %w", err)
	}
	out := make([]domain.CreativeTimelineEvent, 0, len(models))
	for _, m := range models {
		out = append(out, toDomainCreativeTimeline(m))
	}
	return out, nil
}

func toDomainCreativeWorldRule(m creativeWorldRuleModel) domain.CreativeWorldRule {
	return domain.CreativeWorldRule{
		ID: m.ID, CreativeWorldID: m.CreativeWorldID, SourceRuleID: m.SourceRuleID,
		Status: domain.CreativeWorldRuleStatus(m.Status), Category: m.Category,
		Name: m.Name, Description: m.Description, Importance: m.Importance,
		CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}
}

func toDomainCreativeTimeline(m creativeTimelineModel) domain.CreativeTimelineEvent {
	return domain.CreativeTimelineEvent{
		ID: m.ID, CreativeWorkID: m.CreativeWorkID, SourceOriginalEventID: m.SourceOriginalEventID,
		Status: domain.CreativeTimelineEventStatus(m.Status), Sequence: m.Sequence,
		TimeLabel: m.TimeLabel, Title: m.Title, Description: m.Description,
		CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}
}

func toDomainCreativeWork(m creativeWorkModel) domain.CreativeWork {
	w := domain.CreativeWork{
		ID: m.ID, ProjectID: m.ProjectID, OriginalWorkID: m.OriginalWorkID,
		Title: m.Title, Description: m.Description, Status: domain.CreativeWorkStatus(m.Status),
		DivergencePointID: m.DivergencePointID,
		CreatedAt:         m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}
	if m.DeletedAt.Valid {
		t := m.DeletedAt.Time
		w.DeletedAt = &t
	}
	return w
}

func toCreativeCharacterModel(c *domain.CreativeCharacter) (creativeCharacterModel, error) {
	dna, err := json.Marshal(c.DNA)
	if err != nil {
		return creativeCharacterModel{}, fmt.Errorf("序列化二创人物 DNA 失败: %w", err)
	}
	fusionSources, err := json.Marshal(c.FusionSources)
	if err != nil {
		return creativeCharacterModel{}, fmt.Errorf("序列化融合来源失败: %w", err)
	}
	fusionDetail, err := json.Marshal(c.FusionDetail)
	if err != nil {
		return creativeCharacterModel{}, fmt.Errorf("序列化融合说明失败: %w", err)
	}
	modifications, err := json.Marshal(c.Modifications)
	if err != nil {
		return creativeCharacterModel{}, fmt.Errorf("序列化修改说明失败: %w", err)
	}
	return creativeCharacterModel{
		ID: c.ID, CreativeWorkID: c.CreativeWorkID, Name: c.Name, Description: c.Description,
		SourceType: string(c.SourceType), SourceCharacterID: c.SourceCharacterID,
		DNA: string(dna), FusionSources: string(fusionSources), FusionDetail: string(fusionDetail),
		Modifications: string(modifications), Importance: c.Importance, IsLocked: c.IsLocked,
		CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
	}, nil
}

func toDomainCreativeCharacter(m creativeCharacterModel) (domain.CreativeCharacter, error) {
	c := domain.CreativeCharacter{
		ID: m.ID, CreativeWorkID: m.CreativeWorkID, Name: m.Name, Description: m.Description,
		SourceType: domain.CreativeSourceType(m.SourceType), SourceCharacterID: m.SourceCharacterID,
		Importance: m.Importance, IsLocked: m.IsLocked,
		CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
		Modifications: map[string]any{}, FusionSources: []domain.FusionSource{}, FusionDetail: []domain.FusionAttribution{},
	}
	if m.DNA != "" {
		if err := json.Unmarshal([]byte(m.DNA), &c.DNA); err != nil {
			return domain.CreativeCharacter{}, fmt.Errorf("解析二创人物 DNA 失败: %w", err)
		}
	}
	unmarshalJSONB("creative_characters", "fusion_sources", m.FusionSources, &c.FusionSources)
	unmarshalJSONB("creative_characters", "fusion_detail", m.FusionDetail, &c.FusionDetail)
	unmarshalJSONB("creative_characters", "modifications", m.Modifications, &c.Modifications)
	if m.DeletedAt.Valid {
		t := m.DeletedAt.Time
		c.DeletedAt = &t
	}
	return c, nil
}

func toInheritanceModel(r domain.InheritanceRule) inheritanceRuleModel {
	return inheritanceRuleModel{
		ID: r.ID, CreativeCharacterID: r.CreativeCharacterID, SourceCharacterID: r.SourceCharacterID,
		PersonalityWeight: r.PersonalityWeight, ValueWeight: r.ValueWeight,
		MotivationWeight: r.MotivationWeight, BehaviorWeight: r.BehaviorWeight,
		SpeechWeight: r.SpeechWeight, BackgroundWeight: r.BackgroundWeight,
		AbilityWeight: r.AbilityWeight, RelationshipWeight: r.RelationshipWeight,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

func toDomainInheritance(m inheritanceRuleModel) domain.InheritanceRule {
	r := domain.InheritanceRule{
		ID: m.ID, CreativeCharacterID: m.CreativeCharacterID, SourceCharacterID: m.SourceCharacterID,
		PersonalityWeight: m.PersonalityWeight, ValueWeight: m.ValueWeight,
		MotivationWeight: m.MotivationWeight, BehaviorWeight: m.BehaviorWeight,
		SpeechWeight: m.SpeechWeight, BackgroundWeight: m.BackgroundWeight,
		AbilityWeight: m.AbilityWeight, RelationshipWeight: m.RelationshipWeight,
		CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}
	if m.DeletedAt.Valid {
		t := m.DeletedAt.Time
		r.DeletedAt = &t
	}
	return r
}

func toDomainMapping(m mappingModel) domain.OriginalCreativeMapping {
	out := domain.OriginalCreativeMapping{
		ID: m.ID, CreativeWorkID: m.CreativeWorkID,
		OriginalType: m.OriginalType, OriginalID: m.OriginalID,
		CreativeType: m.CreativeType, CreativeID: m.CreativeID,
		MappingType: domain.MappingType(m.MappingType), Description: m.Description,
		CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}
	if m.DeletedAt.Valid {
		t := m.DeletedAt.Time
		out.DeletedAt = &t
	}
	return out
}
