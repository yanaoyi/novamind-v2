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

type worldModel struct {
	ID             string         `gorm:"column:id;type:uuid;primaryKey"`
	OriginalWorkID string         `gorm:"column:original_work_id;type:uuid;not null"`
	Name           string         `gorm:"column:name;size:200;not null;default:''"`
	Description    string         `gorm:"column:description;not null;default:''"`
	CreatedAt      time.Time      `gorm:"column:created_at;not null"`
	UpdatedAt      time.Time      `gorm:"column:updated_at;not null"`
	DeletedAt      gorm.DeletedAt `gorm:"column:deleted_at;index"`
}

func (worldModel) TableName() string { return "original_worlds" }

type worldRuleModel struct {
	ID          string         `gorm:"column:id;type:uuid;primaryKey"`
	WorldID     string         `gorm:"column:world_id;type:uuid;not null"`
	Category    string         `gorm:"column:category;size:60;not null;default:''"`
	Name        string         `gorm:"column:name;size:200;not null"`
	Description string         `gorm:"column:description;not null;default:''"`
	Importance  int            `gorm:"column:importance;not null;default:3"`
	CreatedAt   time.Time      `gorm:"column:created_at;not null"`
	UpdatedAt   time.Time      `gorm:"column:updated_at;not null"`
	DeletedAt   gorm.DeletedAt `gorm:"column:deleted_at;index"`
}

func (worldRuleModel) TableName() string { return "world_rules" }

type locationModel struct {
	ID               string         `gorm:"column:id;type:uuid;primaryKey"`
	WorldID          string         `gorm:"column:world_id;type:uuid;not null"`
	Name             string         `gorm:"column:name;size:200;not null"`
	Type             string         `gorm:"column:type;size:60;not null;default:''"`
	Description      string         `gorm:"column:description;not null;default:''"`
	ParentLocationID *string        `gorm:"column:parent_location_id;type:uuid"`
	CreatedAt        time.Time      `gorm:"column:created_at;not null"`
	UpdatedAt        time.Time      `gorm:"column:updated_at;not null"`
	DeletedAt        gorm.DeletedAt `gorm:"column:deleted_at;index"`
}

func (locationModel) TableName() string { return "locations" }

type factionModel struct {
	ID            string         `gorm:"column:id;type:uuid;primaryKey"`
	WorldID       string         `gorm:"column:world_id;type:uuid;not null"`
	Name          string         `gorm:"column:name;size:200;not null"`
	Type          string         `gorm:"column:type;size:60;not null;default:''"`
	Description   string         `gorm:"column:description;not null;default:''"`
	Goals         string         `gorm:"column:goals;not null;default:''"`
	Relationships string         `gorm:"column:relationships;not null;default:''"`
	CreatedAt     time.Time      `gorm:"column:created_at;not null"`
	UpdatedAt     time.Time      `gorm:"column:updated_at;not null"`
	DeletedAt     gorm.DeletedAt `gorm:"column:deleted_at;index"`
}

func (factionModel) TableName() string { return "factions" }

// OriginalWorldRepo 是世界观相关仓储。
type OriginalWorldRepo struct {
	db *gorm.DB
}

// NewOriginalWorldRepo 构建仓储。
func NewOriginalWorldRepo(db *gorm.DB) *OriginalWorldRepo {
	return &OriginalWorldRepo{db: db}
}

// ---------- 世界 ----------

// UpsertWorld 创建或更新某原著的世界（一个原著一个世界）。
func (r *OriginalWorldRepo) UpsertWorld(ctx context.Context, world *domain.OriginalWorld) error {
	if _, err := uuid.Parse(world.OriginalWorkID); err != nil {
		return domain.ErrOriginalNotFound
	}
	now := time.Now().UTC()

	var existing worldModel
	err := r.db.WithContext(ctx).First(&existing, "original_work_id = ?", world.OriginalWorkID).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		id, genErr := uuid.NewV7()
		if genErr != nil {
			return fmt.Errorf("生成世界 ID 失败: %w", genErr)
		}
		world.ID = id.String()
		world.CreatedAt, world.UpdatedAt = now, now
		m := worldModel{
			ID:             world.ID,
			OriginalWorkID: world.OriginalWorkID,
			Name:           world.Name,
			Description:    world.Description,
			CreatedAt:      world.CreatedAt,
			UpdatedAt:      world.UpdatedAt,
		}
		if err := r.db.WithContext(ctx).Create(&m).Error; err != nil {
			if isForeignKeyViolation(err) {
				return domain.ErrOriginalNotFound
			}
			return fmt.Errorf("创建世界失败: %w", err)
		}
		return nil
	case err != nil:
		return fmt.Errorf("查询世界失败: %w", err)
	}

	res := r.db.WithContext(ctx).Model(&worldModel{}).Where("id = ?", existing.ID).Updates(map[string]any{
		"name":        world.Name,
		"description": world.Description,
		"updated_at":  now,
	})
	if res.Error != nil {
		return fmt.Errorf("更新世界失败: %w", res.Error)
	}
	world.ID = existing.ID
	world.CreatedAt = existing.CreatedAt
	world.UpdatedAt = now
	return nil
}

// GetWorld 按原著取世界。
func (r *OriginalWorldRepo) GetWorld(ctx context.Context, workID string) (*domain.OriginalWorld, error) {
	if _, err := uuid.Parse(workID); err != nil {
		return nil, domain.ErrWorldNotFound
	}
	var m worldModel
	if err := r.db.WithContext(ctx).First(&m, "original_work_id = ?", workID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrWorldNotFound
		}
		return nil, fmt.Errorf("查询世界失败: %w", err)
	}
	w := toDomainWorld(m)
	return &w, nil
}

// GetWorldByID 按世界 ID 取世界。
func (r *OriginalWorldRepo) GetWorldByID(ctx context.Context, id string) (*domain.OriginalWorld, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, domain.ErrWorldNotFound
	}
	var m worldModel
	if err := r.db.WithContext(ctx).First(&m, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrWorldNotFound
		}
		return nil, fmt.Errorf("查询世界失败: %w", err)
	}
	w := toDomainWorld(m)
	return &w, nil
}

// CountWorldChildren 统计世界下的规则/地点/势力数量。
func (r *OriginalWorldRepo) CountWorldChildren(ctx context.Context, worldID string) (rules, locations, factions int64, err error) {
	db := r.db.WithContext(ctx)
	if err = db.Model(&worldRuleModel{}).Where("world_id = ?", worldID).Count(&rules).Error; err != nil {
		return 0, 0, 0, fmt.Errorf("统计规则失败: %w", err)
	}
	if err = db.Model(&locationModel{}).Where("world_id = ?", worldID).Count(&locations).Error; err != nil {
		return 0, 0, 0, fmt.Errorf("统计地点失败: %w", err)
	}
	if err = db.Model(&factionModel{}).Where("world_id = ?", worldID).Count(&factions).Error; err != nil {
		return 0, 0, 0, fmt.Errorf("统计势力失败: %w", err)
	}
	return rules, locations, factions, nil
}

// ---------- 世界规则 ----------

// CreateRule 新增规则。
func (r *OriginalWorldRepo) CreateRule(ctx context.Context, rule *domain.WorldRule) error {
	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("生成规则 ID 失败: %w", err)
	}
	rule.ID = id.String()
	now := time.Now().UTC()
	rule.CreatedAt, rule.UpdatedAt = now, now

	m := worldRuleModel{
		ID: rule.ID, WorldID: rule.WorldID, Category: rule.Category, Name: rule.Name,
		Description: rule.Description, Importance: rule.Importance,
		CreatedAt: rule.CreatedAt, UpdatedAt: rule.UpdatedAt,
	}
	if err := r.db.WithContext(ctx).Create(&m).Error; err != nil {
		if isUniqueViolation(err) {
			return domain.ErrWorldRuleDuplicate
		}
		if isForeignKeyViolation(err) {
			return domain.ErrWorldNotFound
		}
		return fmt.Errorf("创建规则失败: %w", err)
	}
	return nil
}

// ListRules 列出世界的规则。
func (r *OriginalWorldRepo) ListRules(ctx context.Context, worldID string) ([]domain.WorldRule, error) {
	var models []worldRuleModel
	if err := r.db.WithContext(ctx).Where("world_id = ?", worldID).
		Order("importance DESC, created_at ASC").Find(&models).Error; err != nil {
		return nil, fmt.Errorf("查询规则失败: %w", err)
	}
	out := make([]domain.WorldRule, 0, len(models))
	for _, m := range models {
		out = append(out, toDomainRule(m))
	}
	return out, nil
}

// GetRule 取规则。
func (r *OriginalWorldRepo) GetRule(ctx context.Context, id string) (*domain.WorldRule, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, domain.ErrWorldRuleNotFound
	}
	var m worldRuleModel
	if err := r.db.WithContext(ctx).First(&m, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrWorldRuleNotFound
		}
		return nil, fmt.Errorf("查询规则失败: %w", err)
	}
	rule := toDomainRule(m)
	return &rule, nil
}

// UpdateRule 更新规则。
func (r *OriginalWorldRepo) UpdateRule(ctx context.Context, rule *domain.WorldRule) error {
	now := time.Now().UTC()
	res := r.db.WithContext(ctx).Model(&worldRuleModel{}).Where("id = ?", rule.ID).Updates(map[string]any{
		"category":    rule.Category,
		"name":        rule.Name,
		"description": rule.Description,
		"importance":  rule.Importance,
		"updated_at":  now,
	})
	if res.Error != nil {
		if isUniqueViolation(res.Error) {
			return domain.ErrWorldRuleDuplicate
		}
		return fmt.Errorf("更新规则失败: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		if _, err := r.GetRule(ctx, rule.ID); err != nil {
			return err
		}
	}
	rule.UpdatedAt = now
	return nil
}

// DeleteRule 软删除规则。
func (r *OriginalWorldRepo) DeleteRule(ctx context.Context, id string) error {
	if _, err := uuid.Parse(id); err != nil {
		return domain.ErrWorldRuleNotFound
	}
	res := r.db.WithContext(ctx).Where("id = ?", id).Delete(&worldRuleModel{})
	if res.Error != nil {
		return fmt.Errorf("删除规则失败: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.ErrWorldRuleNotFound
	}
	return nil
}

// ---------- 地点 ----------

// CreateLocation 新增地点。
func (r *OriginalWorldRepo) CreateLocation(ctx context.Context, loc *domain.Location) error {
	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("生成地点 ID 失败: %w", err)
	}
	loc.ID = id.String()
	now := time.Now().UTC()
	loc.CreatedAt, loc.UpdatedAt = now, now

	m := locationModel{
		ID: loc.ID, WorldID: loc.WorldID, Name: loc.Name, Type: loc.Type,
		Description: loc.Description, ParentLocationID: loc.ParentLocationID,
		CreatedAt: loc.CreatedAt, UpdatedAt: loc.UpdatedAt,
	}
	if err := r.db.WithContext(ctx).Create(&m).Error; err != nil {
		if isUniqueViolation(err) {
			return domain.ErrLocationDuplicate
		}
		if isForeignKeyViolation(err) {
			return domain.ErrLocationParentCross
		}
		return fmt.Errorf("创建地点失败: %w", err)
	}
	return nil
}

// ListLocations 列出世界的地点。
func (r *OriginalWorldRepo) ListLocations(ctx context.Context, worldID string) ([]domain.Location, error) {
	var models []locationModel
	if err := r.db.WithContext(ctx).Where("world_id = ?", worldID).
		Order("created_at ASC").Find(&models).Error; err != nil {
		return nil, fmt.Errorf("查询地点失败: %w", err)
	}
	out := make([]domain.Location, 0, len(models))
	for _, m := range models {
		out = append(out, toDomainLocation(m))
	}
	return out, nil
}

// GetLocation 取地点。
func (r *OriginalWorldRepo) GetLocation(ctx context.Context, id string) (*domain.Location, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, domain.ErrLocationNotFound
	}
	var m locationModel
	if err := r.db.WithContext(ctx).First(&m, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrLocationNotFound
		}
		return nil, fmt.Errorf("查询地点失败: %w", err)
	}
	loc := toDomainLocation(m)
	return &loc, nil
}

// UpdateLocation 更新地点。
func (r *OriginalWorldRepo) UpdateLocation(ctx context.Context, loc *domain.Location) error {
	now := time.Now().UTC()
	res := r.db.WithContext(ctx).Model(&locationModel{}).Where("id = ?", loc.ID).Updates(map[string]any{
		"name":               loc.Name,
		"type":               loc.Type,
		"description":        loc.Description,
		"parent_location_id": loc.ParentLocationID,
		"updated_at":         now,
	})
	if res.Error != nil {
		if isUniqueViolation(res.Error) {
			return domain.ErrLocationDuplicate
		}
		if isForeignKeyViolation(res.Error) {
			return domain.ErrLocationParentCross
		}
		return fmt.Errorf("更新地点失败: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		if _, err := r.GetLocation(ctx, loc.ID); err != nil {
			return err
		}
	}
	loc.UpdatedAt = now
	return nil
}

// DeleteLocation 软删除地点（子地点的 parent 置空，避免悬挂）。
func (r *OriginalWorldRepo) DeleteLocation(ctx context.Context, id string) error {
	if _, err := uuid.Parse(id); err != nil {
		return domain.ErrLocationNotFound
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&locationModel{}).Where("parent_location_id = ?", id).
			Update("parent_location_id", nil).Error; err != nil {
			return fmt.Errorf("清理子地点失败: %w", err)
		}
		res := tx.Where("id = ?", id).Delete(&locationModel{})
		if res.Error != nil {
			return fmt.Errorf("删除地点失败: %w", res.Error)
		}
		if res.RowsAffected == 0 {
			return domain.ErrLocationNotFound
		}
		return nil
	})
}

// ---------- 势力 ----------

// CreateFaction 新增势力。
func (r *OriginalWorldRepo) CreateFaction(ctx context.Context, f *domain.Faction) error {
	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("生成势力 ID 失败: %w", err)
	}
	f.ID = id.String()
	now := time.Now().UTC()
	f.CreatedAt, f.UpdatedAt = now, now

	m := factionModel{
		ID: f.ID, WorldID: f.WorldID, Name: f.Name, Type: f.Type,
		Description: f.Description, Goals: f.Goals, Relationships: f.Relationships,
		CreatedAt: f.CreatedAt, UpdatedAt: f.UpdatedAt,
	}
	if err := r.db.WithContext(ctx).Create(&m).Error; err != nil {
		if isUniqueViolation(err) {
			return domain.ErrFactionDuplicate
		}
		if isForeignKeyViolation(err) {
			return domain.ErrWorldNotFound
		}
		return fmt.Errorf("创建势力失败: %w", err)
	}
	return nil
}

// ListFactions 列出世界的势力。
func (r *OriginalWorldRepo) ListFactions(ctx context.Context, worldID string) ([]domain.Faction, error) {
	var models []factionModel
	if err := r.db.WithContext(ctx).Where("world_id = ?", worldID).
		Order("created_at ASC").Find(&models).Error; err != nil {
		return nil, fmt.Errorf("查询势力失败: %w", err)
	}
	out := make([]domain.Faction, 0, len(models))
	for _, m := range models {
		out = append(out, toDomainFaction(m))
	}
	return out, nil
}

// GetFaction 取势力。
func (r *OriginalWorldRepo) GetFaction(ctx context.Context, id string) (*domain.Faction, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, domain.ErrFactionNotFound
	}
	var m factionModel
	if err := r.db.WithContext(ctx).First(&m, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrFactionNotFound
		}
		return nil, fmt.Errorf("查询势力失败: %w", err)
	}
	f := toDomainFaction(m)
	return &f, nil
}

// UpdateFaction 更新势力。
func (r *OriginalWorldRepo) UpdateFaction(ctx context.Context, f *domain.Faction) error {
	now := time.Now().UTC()
	res := r.db.WithContext(ctx).Model(&factionModel{}).Where("id = ?", f.ID).Updates(map[string]any{
		"name":          f.Name,
		"type":          f.Type,
		"description":   f.Description,
		"goals":         f.Goals,
		"relationships": f.Relationships,
		"updated_at":    now,
	})
	if res.Error != nil {
		if isUniqueViolation(res.Error) {
			return domain.ErrFactionDuplicate
		}
		return fmt.Errorf("更新势力失败: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		if _, err := r.GetFaction(ctx, f.ID); err != nil {
			return err
		}
	}
	f.UpdatedAt = now
	return nil
}

// DeleteFaction 软删除势力。
func (r *OriginalWorldRepo) DeleteFaction(ctx context.Context, id string) error {
	if _, err := uuid.Parse(id); err != nil {
		return domain.ErrFactionNotFound
	}
	res := r.db.WithContext(ctx).Where("id = ?", id).Delete(&factionModel{})
	if res.Error != nil {
		return fmt.Errorf("删除势力失败: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.ErrFactionNotFound
	}
	return nil
}

// ---------- 转换 ----------

func toDomainWorld(m worldModel) domain.OriginalWorld {
	w := domain.OriginalWorld{
		ID: m.ID, OriginalWorkID: m.OriginalWorkID, Name: m.Name, Description: m.Description,
		CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}
	if m.DeletedAt.Valid {
		t := m.DeletedAt.Time
		w.DeletedAt = &t
	}
	return w
}

func toDomainRule(m worldRuleModel) domain.WorldRule {
	r := domain.WorldRule{
		ID: m.ID, WorldID: m.WorldID, Category: m.Category, Name: m.Name,
		Description: m.Description, Importance: m.Importance,
		CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}
	if m.DeletedAt.Valid {
		t := m.DeletedAt.Time
		r.DeletedAt = &t
	}
	return r
}

func toDomainLocation(m locationModel) domain.Location {
	l := domain.Location{
		ID: m.ID, WorldID: m.WorldID, Name: m.Name, Type: m.Type,
		Description: m.Description, ParentLocationID: m.ParentLocationID,
		CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}
	if m.DeletedAt.Valid {
		t := m.DeletedAt.Time
		l.DeletedAt = &t
	}
	return l
}

func toDomainFaction(m factionModel) domain.Faction {
	f := domain.Faction{
		ID: m.ID, WorldID: m.WorldID, Name: m.Name, Type: m.Type,
		Description: m.Description, Goals: m.Goals, Relationships: m.Relationships,
		CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}
	if m.DeletedAt.Valid {
		t := m.DeletedAt.Time
		f.DeletedAt = &t
	}
	return f
}
