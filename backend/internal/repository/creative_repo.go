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
			return errors.New("工程或原著不存在")
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

// GetWorkByProject 按工程取二创作品。
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
	if err := r.db.WithContext(ctx).Create(&m).Error; err != nil {
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
	m, err := toCreativeCharacterModel(c)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	res := r.db.WithContext(ctx).Model(&creativeCharacterModel{}).Where("id = ?", c.ID).Updates(map[string]any{
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
	var existing inheritanceRuleModel
	err := r.db.WithContext(ctx).
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
		if err := r.db.WithContext(ctx).Create(&m).Error; err != nil {
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
	res := r.db.WithContext(ctx).Model(&inheritanceRuleModel{}).Where("id = ?", existing.ID).Updates(map[string]any{
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
	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("生成映射 ID 失败: %w", err)
	}
	m.ID = id.String()
	now := time.Now().UTC()
	m.CreatedAt, m.UpdatedAt = now, now

	model := mappingModel{
		ID: m.ID, CreativeWorkID: m.CreativeWorkID,
		OriginalType: m.OriginalType, OriginalID: m.OriginalID,
		CreativeType: m.CreativeType, CreativeID: m.CreativeID,
		MappingType: string(m.MappingType), Description: m.Description,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		if isForeignKeyViolation(err) {
			return domain.ErrCreativeNotFound
		}
		return fmt.Errorf("写入映射失败: %w", err)
	}
	return nil
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
	if m.FusionSources != "" {
		_ = json.Unmarshal([]byte(m.FusionSources), &c.FusionSources)
	}
	if m.FusionDetail != "" {
		_ = json.Unmarshal([]byte(m.FusionDetail), &c.FusionDetail)
	}
	if m.Modifications != "" {
		_ = json.Unmarshal([]byte(m.Modifications), &c.Modifications)
	}
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
