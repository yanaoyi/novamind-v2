package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"

	"github.com/yanaoyi/novamindv2/backend/internal/domain"
)

// pgUniqueViolation 是 PostgreSQL 唯一约束冲突的 SQLSTATE。
const pgUniqueViolation = "23505"

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation
}

type characterModel struct {
	ID               string         `gorm:"column:id;type:uuid;primaryKey"`
	OriginalWorkID   string         `gorm:"column:original_work_id;type:uuid;not null"`
	Name             string         `gorm:"column:name;size:120;not null"`
	Aliases          string         `gorm:"column:aliases;type:jsonb;not null;default:'[]'"`
	Role             string         `gorm:"column:role;size:40;not null;default:''"`
	Gender           string         `gorm:"column:gender;size:20;not null;default:''"`
	Age              string         `gorm:"column:age;size:40;not null;default:''"`
	Appearance       string         `gorm:"column:appearance;not null;default:''"`
	Personality      string         `gorm:"column:personality;not null;default:''"`
	Motivation       string         `gorm:"column:motivation;not null;default:''"`
	ValuesText       string         `gorm:"column:values_text;not null;default:''"`
	Fears            string         `gorm:"column:fears;not null;default:''"`
	Desires          string         `gorm:"column:desires;not null;default:''"`
	BehaviorPatterns string         `gorm:"column:behavior_patterns;not null;default:''"`
	SpeechStyle      string         `gorm:"column:speech_style;not null;default:''"`
	Abilities        string         `gorm:"column:abilities;not null;default:''"`
	FirstAppearance  string         `gorm:"column:first_appearance;size:200;not null;default:''"`
	LastAppearance   string         `gorm:"column:last_appearance;size:200;not null;default:''"`
	DNA              string         `gorm:"column:dna;type:jsonb;not null;default:'{}'"`
	Importance       int            `gorm:"column:importance;not null;default:3"`
	Source           string         `gorm:"column:source;size:20;not null;default:MANUAL"`
	Notes            string         `gorm:"column:notes;not null;default:''"`
	CreatedAt        time.Time      `gorm:"column:created_at;not null"`
	UpdatedAt        time.Time      `gorm:"column:updated_at;not null"`
	DeletedAt        gorm.DeletedAt `gorm:"column:deleted_at;index"`
}

func (characterModel) TableName() string { return "original_characters" }

type relationshipModel struct {
	ID                string         `gorm:"column:id;type:uuid;primaryKey"`
	OriginalWorkID    string         `gorm:"column:original_work_id;type:uuid;not null"`
	SourceCharacterID string         `gorm:"column:source_character_id;type:uuid;not null"`
	TargetCharacterID string         `gorm:"column:target_character_id;type:uuid;not null"`
	RelationType      string         `gorm:"column:relation_type;size:20;not null"`
	Strength          int            `gorm:"column:strength;not null;default:50"`
	Description       string         `gorm:"column:description;not null;default:''"`
	Source            string         `gorm:"column:source;size:20;not null;default:MANUAL"`
	CreatedAt         time.Time      `gorm:"column:created_at;not null"`
	UpdatedAt         time.Time      `gorm:"column:updated_at;not null"`
	DeletedAt         gorm.DeletedAt `gorm:"column:deleted_at;index"`
}

func (relationshipModel) TableName() string { return "character_relationships" }

// CharacterFilter 是人物列表查询条件。
type CharacterFilter struct {
	Keyword  string
	Page     int
	PageSize int
}

// OriginalCharacterRepo 是原著人物与关系的仓储。
type OriginalCharacterRepo struct {
	db *gorm.DB
}

// NewOriginalCharacterRepo 构建仓储。
func NewOriginalCharacterRepo(db *gorm.DB) *OriginalCharacterRepo {
	return &OriginalCharacterRepo{db: db}
}

// ---------- 人物 ----------

// CreateCharacter 新增人物；同名（同原著内）返回 domain.ErrCharacterDuplicate。
func (r *OriginalCharacterRepo) CreateCharacter(ctx context.Context, c *domain.OriginalCharacter) error {
	if _, err := uuid.Parse(c.OriginalWorkID); err != nil {
		return domain.ErrOriginalNotFound
	}
	if c.ID == "" {
		id, err := uuid.NewV7()
		if err != nil {
			return fmt.Errorf("生成人物 ID 失败: %w", err)
		}
		c.ID = id.String()
	}
	now := time.Now().UTC()
	c.CreatedAt, c.UpdatedAt = now, now

	m, err := toCharacterModel(c)
	if err != nil {
		return err
	}
	if err := r.db.WithContext(ctx).Create(&m).Error; err != nil {
		if isUniqueViolation(err) {
			return domain.ErrCharacterDuplicate
		}
		if isForeignKeyViolation(err) {
			return domain.ErrOriginalNotFound
		}
		return fmt.Errorf("创建人物失败: %w", err)
	}
	return nil
}

// GetCharacter 按 ID 取人物。
func (r *OriginalCharacterRepo) GetCharacter(ctx context.Context, id string) (*domain.OriginalCharacter, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, domain.ErrCharacterNotFound
	}
	var m characterModel
	if err := r.db.WithContext(ctx).First(&m, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrCharacterNotFound
		}
		return nil, fmt.Errorf("查询人物失败: %w", err)
	}
	c, err := toDomainCharacter(m)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// ListCharacters 分页列出某原著的人物。
func (r *OriginalCharacterRepo) ListCharacters(ctx context.Context, workID string, f CharacterFilter) ([]domain.OriginalCharacter, int64, error) {
	if _, err := uuid.Parse(workID); err != nil {
		return nil, 0, domain.ErrOriginalNotFound
	}
	query := r.db.WithContext(ctx).Model(&characterModel{}).Where("original_work_id = ?", workID)
	if f.Keyword != "" {
		query = query.Where("name ILIKE ? OR role ILIKE ?", "%"+f.Keyword+"%", "%"+f.Keyword+"%")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("统计人物失败: %w", err)
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

	var models []characterModel
	if err := query.Order("importance DESC, created_at ASC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&models).Error; err != nil {
		return nil, 0, fmt.Errorf("查询人物失败: %w", err)
	}

	out := make([]domain.OriginalCharacter, 0, len(models))
	for _, m := range models {
		c, err := toDomainCharacter(m)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, c)
	}
	return out, total, nil
}

// UpdateCharacter 更新人物（改名冲突返回 ErrCharacterDuplicate）。
func (r *OriginalCharacterRepo) UpdateCharacter(ctx context.Context, c *domain.OriginalCharacter) error {
	m, err := toCharacterModel(c)
	if err != nil {
		return err
	}
	m.UpdatedAt = time.Now().UTC()

	res := r.db.WithContext(ctx).Model(&characterModel{}).Where("id = ?", c.ID).Updates(map[string]any{
		"name":              m.Name,
		"aliases":           m.Aliases,
		"role":              m.Role,
		"gender":            m.Gender,
		"age":               m.Age,
		"appearance":        m.Appearance,
		"personality":       m.Personality,
		"motivation":        m.Motivation,
		"values_text":       m.ValuesText,
		"fears":             m.Fears,
		"desires":           m.Desires,
		"behavior_patterns": m.BehaviorPatterns,
		"speech_style":      m.SpeechStyle,
		"abilities":         m.Abilities,
		"first_appearance":  m.FirstAppearance,
		"last_appearance":   m.LastAppearance,
		"dna":               m.DNA,
		"importance":        m.Importance,
		"notes":             m.Notes,
		"updated_at":        m.UpdatedAt,
	})
	if res.Error != nil {
		if isUniqueViolation(res.Error) {
			return domain.ErrCharacterDuplicate
		}
		return fmt.Errorf("更新人物失败: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		if _, err := r.GetCharacter(ctx, c.ID); err != nil {
			return err
		}
	}
	c.UpdatedAt = m.UpdatedAt
	return nil
}

// DeleteCharacter 软删除人物，并连带软删除与其相关的关系。
func (r *OriginalCharacterRepo) DeleteCharacter(ctx context.Context, id string) error {
	if _, err := uuid.Parse(id); err != nil {
		return domain.ErrCharacterNotFound
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Where("id = ?", id).Delete(&characterModel{})
		if res.Error != nil {
			return fmt.Errorf("删除人物失败: %w", res.Error)
		}
		if res.RowsAffected == 0 {
			return domain.ErrCharacterNotFound
		}
		if err := tx.Where("source_character_id = ? OR target_character_id = ?", id, id).
			Delete(&relationshipModel{}).Error; err != nil {
			return fmt.Errorf("清理人物关系失败: %w", err)
		}
		return nil
	})
}

// ---------- 人物关系 ----------

// CreateRelationship 新增关系。
func (r *OriginalCharacterRepo) CreateRelationship(ctx context.Context, rel *domain.CharacterRelationship) error {
	if rel.ID == "" {
		id, err := uuid.NewV7()
		if err != nil {
			return fmt.Errorf("生成关系 ID 失败: %w", err)
		}
		rel.ID = id.String()
	}
	now := time.Now().UTC()
	rel.CreatedAt, rel.UpdatedAt = now, now

	m := relationshipModel{
		ID:                rel.ID,
		OriginalWorkID:    rel.OriginalWorkID,
		SourceCharacterID: rel.SourceCharacterID,
		TargetCharacterID: rel.TargetCharacterID,
		RelationType:      string(rel.RelationType),
		Strength:          rel.Strength,
		Description:       rel.Description,
		Source:            string(rel.Source),
		CreatedAt:         rel.CreatedAt,
		UpdatedAt:         rel.UpdatedAt,
	}
	if err := r.db.WithContext(ctx).Create(&m).Error; err != nil {
		if isUniqueViolation(err) {
			return domain.ErrRelationDuplicate
		}
		if isForeignKeyViolation(err) {
			return domain.ErrCharacterNotFound
		}
		return fmt.Errorf("创建人物关系失败: %w", err)
	}
	return nil
}

// ListRelationships 列出某原著的全部关系（V1 不做分页：关系数量级有限）。
func (r *OriginalCharacterRepo) ListRelationships(ctx context.Context, workID string) ([]domain.CharacterRelationship, error) {
	if _, err := uuid.Parse(workID); err != nil {
		return nil, domain.ErrOriginalNotFound
	}
	var models []relationshipModel
	if err := r.db.WithContext(ctx).
		Where("original_work_id = ?", workID).
		Order("created_at ASC").Find(&models).Error; err != nil {
		return nil, fmt.Errorf("查询人物关系失败: %w", err)
	}
	out := make([]domain.CharacterRelationship, 0, len(models))
	for _, m := range models {
		out = append(out, toDomainRelationship(m))
	}
	return out, nil
}

// GetRelationship 按 ID 取关系。
func (r *OriginalCharacterRepo) GetRelationship(ctx context.Context, id string) (*domain.CharacterRelationship, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, domain.ErrRelationNotFound
	}
	var m relationshipModel
	if err := r.db.WithContext(ctx).First(&m, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrRelationNotFound
		}
		return nil, fmt.Errorf("查询人物关系失败: %w", err)
	}
	rel := toDomainRelationship(m)
	return &rel, nil
}

// UpdateRelationship 更新关系。
func (r *OriginalCharacterRepo) UpdateRelationship(ctx context.Context, rel *domain.CharacterRelationship) error {
	now := time.Now().UTC()
	res := r.db.WithContext(ctx).Model(&relationshipModel{}).Where("id = ?", rel.ID).Updates(map[string]any{
		"relation_type": string(rel.RelationType),
		"strength":      rel.Strength,
		"description":   rel.Description,
		"updated_at":    now,
	})
	if res.Error != nil {
		if isUniqueViolation(res.Error) {
			return domain.ErrRelationDuplicate
		}
		return fmt.Errorf("更新人物关系失败: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		if _, err := r.GetRelationship(ctx, rel.ID); err != nil {
			return err
		}
	}
	rel.UpdatedAt = now
	return nil
}

// DeleteRelationship 软删除关系。
func (r *OriginalCharacterRepo) DeleteRelationship(ctx context.Context, id string) error {
	if _, err := uuid.Parse(id); err != nil {
		return domain.ErrRelationNotFound
	}
	res := r.db.WithContext(ctx).Where("id = ?", id).Delete(&relationshipModel{})
	if res.Error != nil {
		return fmt.Errorf("删除人物关系失败: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.ErrRelationNotFound
	}
	return nil
}

// ---------- 转换 ----------

func toCharacterModel(c *domain.OriginalCharacter) (characterModel, error) {
	aliases, err := json.Marshal(c.Aliases)
	if err != nil {
		return characterModel{}, fmt.Errorf("序列化别名失败: %w", err)
	}
	if c.Aliases == nil {
		aliases = []byte("[]")
	}
	dna, err := json.Marshal(c.DNA)
	if err != nil {
		return characterModel{}, fmt.Errorf("序列化人物 DNA 失败: %w", err)
	}

	return characterModel{
		ID:               c.ID,
		OriginalWorkID:   c.OriginalWorkID,
		Name:             c.Name,
		Aliases:          string(aliases),
		Role:             c.Role,
		Gender:           c.Gender,
		Age:              c.Age,
		Appearance:       c.Appearance,
		Personality:      c.Personality,
		Motivation:       c.Motivation,
		ValuesText:       c.Values,
		Fears:            c.Fears,
		Desires:          c.Desires,
		BehaviorPatterns: c.BehaviorPatterns,
		SpeechStyle:      c.SpeechStyle,
		Abilities:        c.Abilities,
		FirstAppearance:  c.FirstAppearance,
		LastAppearance:   c.LastAppearance,
		DNA:              string(dna),
		Importance:       c.Importance,
		Source:           string(c.Source),
		Notes:            c.Notes,
		CreatedAt:        c.CreatedAt,
		UpdatedAt:        c.UpdatedAt,
	}, nil
}

func toDomainCharacter(m characterModel) (domain.OriginalCharacter, error) {
	c := domain.OriginalCharacter{
		ID:               m.ID,
		OriginalWorkID:   m.OriginalWorkID,
		Name:             m.Name,
		Role:             m.Role,
		Gender:           m.Gender,
		Age:              m.Age,
		Appearance:       m.Appearance,
		Personality:      m.Personality,
		Motivation:       m.Motivation,
		Values:           m.ValuesText,
		Fears:            m.Fears,
		Desires:          m.Desires,
		BehaviorPatterns: m.BehaviorPatterns,
		SpeechStyle:      m.SpeechStyle,
		Abilities:        m.Abilities,
		FirstAppearance:  m.FirstAppearance,
		LastAppearance:   m.LastAppearance,
		Importance:       m.Importance,
		Source:           domain.CharacterSource(m.Source),
		Notes:            m.Notes,
		CreatedAt:        m.CreatedAt,
		UpdatedAt:        m.UpdatedAt,
	}
	if m.Aliases != "" {
		if err := json.Unmarshal([]byte(m.Aliases), &c.Aliases); err != nil {
			return domain.OriginalCharacter{}, fmt.Errorf("解析别名失败: %w", err)
		}
	}
	if c.Aliases == nil {
		c.Aliases = []string{}
	}
	if m.DNA != "" {
		if err := json.Unmarshal([]byte(m.DNA), &c.DNA); err != nil {
			return domain.OriginalCharacter{}, fmt.Errorf("解析人物 DNA 失败: %w", err)
		}
	}
	if m.DeletedAt.Valid {
		t := m.DeletedAt.Time
		c.DeletedAt = &t
	}
	return c, nil
}

func toDomainRelationship(m relationshipModel) domain.CharacterRelationship {
	rel := domain.CharacterRelationship{
		ID:                m.ID,
		OriginalWorkID:    m.OriginalWorkID,
		SourceCharacterID: m.SourceCharacterID,
		TargetCharacterID: m.TargetCharacterID,
		RelationType:      domain.RelationType(m.RelationType),
		Strength:          m.Strength,
		Description:       m.Description,
		Source:            domain.CharacterSource(m.Source),
		CreatedAt:         m.CreatedAt,
		UpdatedAt:         m.UpdatedAt,
	}
	if m.DeletedAt.Valid {
		t := m.DeletedAt.Time
		rel.DeletedAt = &t
	}
	return rel
}

func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}

// isDuplicateKey 判断唯一约束冲突（PostgreSQL 23505）。
func isDuplicateKey(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
