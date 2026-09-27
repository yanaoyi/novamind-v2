package service

import (
	"context"
	"strings"

	"github.com/yanaoyi/novamindv2/backend/internal/domain"
)

// OriginalWorldRepository 是世界观 service 需要的仓储能力。
type OriginalWorldRepository interface {
	UpsertWorld(ctx context.Context, w *domain.OriginalWorld) error
	GetWorld(ctx context.Context, workID string) (*domain.OriginalWorld, error)
	GetWorldByID(ctx context.Context, id string) (*domain.OriginalWorld, error)
	CountWorldChildren(ctx context.Context, worldID string) (int64, int64, int64, error)

	CreateRule(ctx context.Context, r *domain.WorldRule) error
	ListRules(ctx context.Context, worldID string) ([]domain.WorldRule, error)
	GetRule(ctx context.Context, id string) (*domain.WorldRule, error)
	UpdateRule(ctx context.Context, r *domain.WorldRule) error
	DeleteRule(ctx context.Context, id string) error

	CreateLocation(ctx context.Context, l *domain.Location) error
	ListLocations(ctx context.Context, worldID string) ([]domain.Location, error)
	GetLocation(ctx context.Context, id string) (*domain.Location, error)
	UpdateLocation(ctx context.Context, l *domain.Location) error
	DeleteLocation(ctx context.Context, id string) error

	CreateFaction(ctx context.Context, f *domain.Faction) error
	ListFactions(ctx context.Context, worldID string) ([]domain.Faction, error)
	GetFaction(ctx context.Context, id string) (*domain.Faction, error)
	UpdateFaction(ctx context.Context, f *domain.Faction) error
	DeleteFaction(ctx context.Context, id string) error
}

// OriginalWorldService 是原著世界观的业务服务。
type OriginalWorldService struct {
	repo  OriginalWorldRepository
	works WorkLookup
}

// NewOriginalWorldService 构建服务。
func NewOriginalWorldService(repo OriginalWorldRepository, works WorkLookup) *OriginalWorldService {
	return &OriginalWorldService{repo: repo, works: works}
}

// UpsertWorld 创建或更新某原著的世界。
func (s *OriginalWorldService) UpsertWorld(ctx context.Context, workID, name, description string) (*domain.OriginalWorld, error) {
	if _, err := s.works.GetWorkByID(ctx, workID); err != nil {
		return nil, err
	}
	w := &domain.OriginalWorld{OriginalWorkID: workID, Name: name, Description: description}
	w.Normalize()
	if err := s.repo.UpsertWorld(ctx, w); err != nil {
		return nil, err
	}
	return w, nil
}

// GetSummary 取世界观及其规模统计；世界还不存在时返回 ErrWorldNotFound。
func (s *OriginalWorldService) GetSummary(ctx context.Context, workID string) (*domain.WorldSummary, error) {
	if _, err := s.works.GetWorkByID(ctx, workID); err != nil {
		return nil, err
	}
	world, err := s.repo.GetWorld(ctx, workID)
	if err != nil {
		return nil, err
	}
	rules, locations, factions, err := s.repo.CountWorldChildren(ctx, world.ID)
	if err != nil {
		return nil, err
	}
	return &domain.WorldSummary{
		World:     *world,
		Rules:     int(rules),
		Locations: int(locations),
		Factions:  int(factions),
	}, nil
}

// ensureWorld 保证世界存在（不存在则建一个空世界），供规则/地点/势力直接新增时使用。
func (s *OriginalWorldService) ensureWorld(ctx context.Context, workID string) (*domain.OriginalWorld, error) {
	if _, err := s.works.GetWorkByID(ctx, workID); err != nil {
		return nil, err
	}
	world, err := s.repo.GetWorld(ctx, workID)
	if err == nil {
		return world, nil
	}
	if err != domain.ErrWorldNotFound {
		return nil, err
	}
	created := &domain.OriginalWorld{OriginalWorkID: workID}
	if err := s.repo.UpsertWorld(ctx, created); err != nil {
		return nil, err
	}
	return created, nil
}

// ---------- 世界规则 ----------

// RuleInput 是规则入参。
type RuleInput struct {
	Category    string
	Name        string
	Description string
	Importance  int
}

// CreateRule 新增规则（世界不存在时自动创建空世界）。
func (s *OriginalWorldService) CreateRule(ctx context.Context, workID string, in RuleInput) (*domain.WorldRule, error) {
	world, err := s.ensureWorld(ctx, workID)
	if err != nil {
		return nil, err
	}
	rule := &domain.WorldRule{
		WorldID:     world.ID,
		Category:    in.Category,
		Name:        in.Name,
		Description: in.Description,
		Importance:  in.Importance,
	}
	rule.Normalize()
	if err := rule.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.CreateRule(ctx, rule); err != nil {
		return nil, err
	}
	return rule, nil
}

// ListRules 列出某原著的规则。
func (s *OriginalWorldService) ListRules(ctx context.Context, workID string) ([]domain.WorldRule, error) {
	world, err := s.repo.GetWorld(ctx, workID)
	if err != nil {
		return []domain.WorldRule{}, nil // 世界还没建时返回空列表而不是 404
	}
	items, err := s.repo.ListRules(ctx, world.ID)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []domain.WorldRule{}
	}
	return items, nil
}

// UpdateRule 更新规则。
func (s *OriginalWorldService) UpdateRule(ctx context.Context, id string, in RuleInput) (*domain.WorldRule, error) {
	rule, err := s.repo.GetRule(ctx, id)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.Name) != "" {
		rule.Name = in.Name
	}
	rule.Category = in.Category
	rule.Description = in.Description
	if in.Importance > 0 {
		rule.Importance = in.Importance
	}
	rule.Normalize()
	if err := rule.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateRule(ctx, rule); err != nil {
		return nil, err
	}
	return rule, nil
}

// DeleteRule 删除规则。
func (s *OriginalWorldService) DeleteRule(ctx context.Context, id string) error {
	return s.repo.DeleteRule(ctx, id)
}

// ---------- 地点 ----------

// LocationInput 是地点入参。
type LocationInput struct {
	Name             string
	Type             string
	Description      string
	ParentLocationID *string
}

// CreateLocation 新增地点，并校验上级地点同世界、无环。
func (s *OriginalWorldService) CreateLocation(ctx context.Context, workID string, in LocationInput) (*domain.Location, error) {
	world, err := s.ensureWorld(ctx, workID)
	if err != nil {
		return nil, err
	}
	loc := &domain.Location{
		WorldID:          world.ID,
		Name:             in.Name,
		Type:             in.Type,
		Description:      in.Description,
		ParentLocationID: in.ParentLocationID,
	}
	loc.Normalize()
	if err := loc.Validate(); err != nil {
		return nil, err
	}
	if loc.ParentLocationID != nil {
		parent, err := s.repo.GetLocation(ctx, *loc.ParentLocationID)
		if err != nil {
			return nil, err
		}
		if parent.WorldID != world.ID {
			return nil, domain.ErrLocationParentCross
		}
	}
	if err := s.repo.CreateLocation(ctx, loc); err != nil {
		return nil, err
	}
	return loc, nil
}

// ListLocations 列出某原著的地点。
func (s *OriginalWorldService) ListLocations(ctx context.Context, workID string) ([]domain.Location, error) {
	world, err := s.repo.GetWorld(ctx, workID)
	if err != nil {
		return []domain.Location{}, nil
	}
	items, err := s.repo.ListLocations(ctx, world.ID)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []domain.Location{}
	}
	return items, nil
}

// UpdateLocation 更新地点；改上级时校验同世界且不形成环。
func (s *OriginalWorldService) UpdateLocation(ctx context.Context, id string, in LocationInput) (*domain.Location, error) {
	loc, err := s.repo.GetLocation(ctx, id)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.Name) != "" {
		loc.Name = in.Name
	}
	loc.Type = in.Type
	loc.Description = in.Description
	loc.ParentLocationID = in.ParentLocationID
	loc.Normalize()
	if err := loc.Validate(); err != nil {
		return nil, err
	}
	if loc.ParentLocationID != nil {
		if err := s.assertNoCycle(ctx, loc.ID, *loc.ParentLocationID, loc.WorldID); err != nil {
			return nil, err
		}
	}
	if err := s.repo.UpdateLocation(ctx, loc); err != nil {
		return nil, err
	}
	return loc, nil
}

// DeleteLocation 删除地点。
func (s *OriginalWorldService) DeleteLocation(ctx context.Context, id string) error {
	return s.repo.DeleteLocation(ctx, id)
}

// assertNoCycle 沿上级链向上走，若回到自己或超出深度上限则判定成环。
func (s *OriginalWorldService) assertNoCycle(ctx context.Context, selfID, parentID, worldID string) error {
	if selfID == parentID {
		return domain.ErrLocationSelfParent
	}
	seen := map[string]bool{selfID: true}
	current := parentID
	for depth := 0; depth < 100; depth++ {
		if seen[current] {
			return domain.ErrLocationCycle
		}
		seen[current] = true
		parent, err := s.repo.GetLocation(ctx, current)
		if err != nil {
			return err
		}
		if parent.WorldID != worldID {
			return domain.ErrLocationParentCross
		}
		if parent.ParentLocationID == nil {
			return nil
		}
		current = *parent.ParentLocationID
	}
	return domain.ErrLocationCycle
}

// ---------- 势力 ----------

// FactionInput 是势力入参。
type FactionInput struct {
	Name          string
	Type          string
	Description   string
	Goals         string
	Relationships string
}

// CreateFaction 新增势力。
func (s *OriginalWorldService) CreateFaction(ctx context.Context, workID string, in FactionInput) (*domain.Faction, error) {
	world, err := s.ensureWorld(ctx, workID)
	if err != nil {
		return nil, err
	}
	faction := &domain.Faction{
		WorldID:       world.ID,
		Name:          in.Name,
		Type:          in.Type,
		Description:   in.Description,
		Goals:         in.Goals,
		Relationships: in.Relationships,
	}
	faction.Normalize()
	if err := faction.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.CreateFaction(ctx, faction); err != nil {
		return nil, err
	}
	return faction, nil
}

// ListFactions 列出某原著的势力。
func (s *OriginalWorldService) ListFactions(ctx context.Context, workID string) ([]domain.Faction, error) {
	world, err := s.repo.GetWorld(ctx, workID)
	if err != nil {
		return []domain.Faction{}, nil
	}
	items, err := s.repo.ListFactions(ctx, world.ID)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []domain.Faction{}
	}
	return items, nil
}

// UpdateFaction 更新势力。
func (s *OriginalWorldService) UpdateFaction(ctx context.Context, id string, in FactionInput) (*domain.Faction, error) {
	faction, err := s.repo.GetFaction(ctx, id)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.Name) != "" {
		faction.Name = in.Name
	}
	faction.Type = in.Type
	faction.Description = in.Description
	faction.Goals = in.Goals
	faction.Relationships = in.Relationships
	faction.Normalize()
	if err := faction.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateFaction(ctx, faction); err != nil {
		return nil, err
	}
	return faction, nil
}

// DeleteFaction 删除势力。
func (s *OriginalWorldService) DeleteFaction(ctx context.Context, id string) error {
	return s.repo.DeleteFaction(ctx, id)
}
