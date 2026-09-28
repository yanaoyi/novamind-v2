package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/yanaoyi/novamindv2/backend/internal/domain"
)

// CreativeRepository 是二创 service 需要的仓储能力。
type CreativeRepository interface {
	CreateWork(ctx context.Context, w *domain.CreativeWork) error
	GetWorkByID(ctx context.Context, id string) (*domain.CreativeWork, error)
	GetWorkByProject(ctx context.Context, projectID string) (*domain.CreativeWork, error)
	ListWorksByOriginal(ctx context.Context, originalWorkID string) ([]domain.CreativeWork, error)
	UpdateWork(ctx context.Context, w *domain.CreativeWork) error
	SetDivergencePoint(ctx context.Context, workID, divergencePointID string) error

	CreateCharacter(ctx context.Context, c *domain.CreativeCharacter) error
	GetCharacter(ctx context.Context, id string) (*domain.CreativeCharacter, error)
	ListCharacters(ctx context.Context, workID string) ([]domain.CreativeCharacter, error)
	UpdateCharacter(ctx context.Context, c *domain.CreativeCharacter) error
	DeleteCharacter(ctx context.Context, id string) error

	UpsertInheritanceRule(ctx context.Context, rule *domain.InheritanceRule) error
	ListInheritanceRules(ctx context.Context, creativeCharacterID string) ([]domain.InheritanceRule, error)

	CreateMapping(ctx context.Context, m *domain.OriginalCreativeMapping) error
	ListMappings(ctx context.Context, workID string) ([]domain.OriginalCreativeMapping, error)
	DeleteMapping(ctx context.Context, id string) error

	UpsertWorld(ctx context.Context, w *domain.CreativeWorld) error
	GetWorld(ctx context.Context, creativeWorkID string) (*domain.CreativeWorld, error)
	GetWorkIDByCreativeWorld(ctx context.Context, creativeWorldID string) (string, error)
	CreateWorldRule(ctx context.Context, rule *domain.CreativeWorldRule) error
	ListWorldRules(ctx context.Context, creativeWorldID string) ([]domain.CreativeWorldRule, error)
	GetWorldRule(ctx context.Context, id string) (*domain.CreativeWorldRule, error)
	UpdateWorldRule(ctx context.Context, rule *domain.CreativeWorldRule) error
	DeleteWorldRule(ctx context.Context, id string) error
	UpsertDivergence(ctx context.Context, d *domain.DivergencePoint) error
	GetDivergence(ctx context.Context, creativeWorkID string) (*domain.DivergencePoint, error)
	ReplaceTimeline(ctx context.Context, creativeWorkID string, events []domain.CreativeTimelineEvent) error
	ListTimeline(ctx context.Context, creativeWorkID string) ([]domain.CreativeTimelineEvent, error)
}

// OriginalWorldReader 读取原著世界观（继承来源）。
type OriginalWorldReader interface {
	GetWorld(ctx context.Context, workID string) (*domain.OriginalWorld, error)
	ListRules(ctx context.Context, worldID string) ([]domain.WorldRule, error)
}

// OriginalTimelineReader 读取原著时间线与事件（继承来源）。
type OriginalTimelineReader interface {
	GetTimeline(ctx context.Context, workID string) (*domain.OriginalTimeline, error)
	ListTimelineEntries(ctx context.Context, timelineID string) ([]domain.TimelineEntry, error)
	GetEvent(ctx context.Context, id string) (*domain.OriginalEvent, error)
}

// CharacterLookup 用于读取原著人物（继承来源）。
type CharacterSourceLookup interface {
	GetCharacter(ctx context.Context, id string) (*domain.OriginalCharacter, error)
}

// CreativeService 是二创作品的业务服务。
type CreativeService struct {
	repo          CreativeRepository
	projects      ProjectRepository
	works         WorkLookup
	characters    CharacterSourceLookup
	origWorlds    OriginalWorldReader
	origTimelines OriginalTimelineReader
}

// NewCreativeService 构建服务。
func NewCreativeService(
	repo CreativeRepository,
	projects ProjectRepository,
	works WorkLookup,
	characters CharacterSourceLookup,
	origWorlds OriginalWorldReader,
	origTimelines OriginalTimelineReader,
) *CreativeService {
	return &CreativeService{
		repo: repo, projects: projects, works: works, characters: characters,
		origWorlds: origWorlds, origTimelines: origTimelines,
	}
}

// CreateWorkInput 是创建二创作品的入参。
type CreateWorkInput struct {
	ProjectID      string
	OriginalWorkID string
	Title          string
	Description    string
}

// CreateWork 从原著创建二创作品。
func (s *CreativeService) CreateWork(ctx context.Context, in CreateWorkInput) (*domain.CreativeWork, error) {
	project, err := s.projects.GetByID(ctx, in.ProjectID)
	if err != nil {
		return nil, err
	}
	if project.Type != domain.ProjectTypeCreative {
		return nil, domain.ErrCreativeNotCreativeProj
	}
	if _, err := s.works.GetWorkByID(ctx, in.OriginalWorkID); err != nil {
		return nil, err
	}
	if _, err := s.repo.GetWorkByProject(ctx, in.ProjectID); err == nil {
		return nil, domain.ErrCreativeAlreadyExists
	} else if !errors.Is(err, domain.ErrCreativeNotFound) {
		return nil, err
	}

	w := &domain.CreativeWork{
		ProjectID: in.ProjectID, OriginalWorkID: in.OriginalWorkID,
		Title: in.Title, Description: in.Description,
	}
	w.Normalize()
	if err := w.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.CreateWork(ctx, w); err != nil {
		return nil, err
	}
	return w, nil
}

// GetWork 取二创作品。
func (s *CreativeService) GetWork(ctx context.Context, id string) (*domain.CreativeWork, error) {
	if strings.TrimSpace(id) == "" {
		return nil, domain.ErrCreativeNotFound
	}
	return s.repo.GetWorkByID(ctx, id)
}

// GetWorkByProject 按工程取二创作品。
func (s *CreativeService) GetWorkByProject(ctx context.Context, projectID string) (*domain.CreativeWork, error) {
	return s.repo.GetWorkByProject(ctx, projectID)
}

// ListWorksByOriginal 列出某原著下的二创作品。
func (s *CreativeService) ListWorksByOriginal(ctx context.Context, originalWorkID string) ([]domain.CreativeWork, error) {
	items, err := s.repo.ListWorksByOriginal(ctx, originalWorkID)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []domain.CreativeWork{}
	}
	return items, nil
}

// UpdateWorkInput 是更新二创作品的入参。
type UpdateWorkInput struct {
	Title       *string
	Description *string
	Status      *domain.CreativeWorkStatus
}

// UpdateWork 更新二创作品。
func (s *CreativeService) UpdateWork(ctx context.Context, id string, in UpdateWorkInput) (*domain.CreativeWork, error) {
	w, err := s.repo.GetWorkByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if in.Title != nil {
		w.Title = *in.Title
	}
	if in.Description != nil {
		w.Description = *in.Description
	}
	if in.Status != nil {
		w.Status = *in.Status
	}
	w.Normalize()
	if err := w.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateWork(ctx, w); err != nil {
		return nil, err
	}
	return w, nil
}

// InheritInput 是人物继承入参。
type InheritInput struct {
	SourceCharacterID string
	Name              string // 可空：默认沿用原著人物姓名
	Description       string
	Importance        int
	Rule              domain.InheritanceRule
}

// InheritCharacter 从原著人物继承出一个二创人物。
//
// 流程：校验来源人物属于该二创作品依据的原著 → 按权重算出 DNA → 建人物 → 记继承规则 → 记映射。
// 同名二创人物已存在时，视为"重新继承"（前提是没被锁定）。
func (s *CreativeService) InheritCharacter(ctx context.Context, workID string, in InheritInput) (*domain.CreativeCharacter, error) {
	work, err := s.repo.GetWorkByID(ctx, workID)
	if err != nil {
		return nil, err
	}
	source, err := s.characters.GetCharacter(ctx, in.SourceCharacterID)
	if err != nil {
		if errors.Is(err, domain.ErrCharacterNotFound) {
			return nil, domain.ErrSourceCharacterNotFound
		}
		return nil, err
	}
	if source.OriginalWorkID != work.OriginalWorkID {
		return nil, domain.ErrSourceCharacterCrossWork
	}

	rule := in.Rule
	if err := rule.Validate(); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = source.Name
	}

	sourceType := domain.SourceOriginalInherited
	if name != source.Name || strings.TrimSpace(in.Description) != "" {
		sourceType = domain.SourceModified
	}

	character := &domain.CreativeCharacter{
		CreativeWorkID:    work.ID,
		Name:              name,
		Description:       in.Description,
		SourceType:        sourceType,
		SourceCharacterID: &source.ID,
		DNA:               domain.ApplyInheritance(source.DNA, rule),
		Importance:        in.Importance,
	}
	character.Normalize()
	if err := character.Validate(); err != nil {
		return nil, err
	}

	// 同名已存在：重新继承（未锁定才允许）
	if existing, err := s.findByWorkAndName(ctx, work.ID, name); err != nil {
		return nil, err
	} else if existing != nil {
		if existing.IsLocked {
			return nil, domain.ErrCreativeCharacterLocked
		}
		character.ID = existing.ID
		character.CreatedAt = existing.CreatedAt
		if err := s.repo.UpdateCharacter(ctx, character); err != nil {
			return nil, err
		}
	} else if err := s.repo.CreateCharacter(ctx, character); err != nil {
		return nil, err
	}

	rule.CreativeCharacterID = character.ID
	rule.SourceCharacterID = source.ID
	if err := s.repo.UpsertInheritanceRule(ctx, &rule); err != nil {
		return nil, err
	}

	// 映射：原著人物 → 二创人物
	mappingType := domain.MappingInherited
	if sourceType == domain.SourceModified {
		mappingType = domain.MappingModified
	}
	if err := s.repo.CreateMapping(ctx, &domain.OriginalCreativeMapping{
		CreativeWorkID: work.ID,
		OriginalType:   "character", OriginalID: source.ID,
		CreativeType: "creative_character", CreativeID: character.ID,
		MappingType: mappingType,
		Description: fmt.Sprintf("继承自原著人物「%s」", source.Name),
	}); err != nil {
		return nil, err
	}
	return character, nil
}

// CreateNewCharacterInput 是原创人物入参。
type CreateNewCharacterInput struct {
	Name        string
	Description string
	DNA         domain.CharacterDNA
	Importance  int
}

// CreateNewCharacter 创建一个与原著无关的原创人物（source_type=NEW，不产生映射）。
func (s *CreativeService) CreateNewCharacter(ctx context.Context, workID string, in CreateNewCharacterInput) (*domain.CreativeCharacter, error) {
	if _, err := s.repo.GetWorkByID(ctx, workID); err != nil {
		return nil, err
	}
	character := &domain.CreativeCharacter{
		CreativeWorkID: workID, Name: in.Name, Description: in.Description,
		SourceType: domain.SourceNew, DNA: in.DNA, Importance: in.Importance,
	}
	character.Normalize()
	if err := character.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.CreateCharacter(ctx, character); err != nil {
		return nil, err
	}
	return character, nil
}

// FusionInputItem 是融合的一个来源（用二创人物做来源，便于"先继承再融合"）。
type FusionInputItem struct {
	CharacterID string
	Weight      int
}

// FuseInput 是人物融合入参。
type FuseInput struct {
	Name        string
	Description string
	Importance  int
	Sources     []FusionInputItem
}

// FuseCharacter 把多个二创人物融合成一个新人物，并留下可追溯的融合说明。
func (s *CreativeService) FuseCharacter(ctx context.Context, workID string, in FuseInput) (*domain.CreativeCharacter, error) {
	if len(in.Sources) < 2 {
		return nil, domain.ErrFusionNeedsTwoSources
	}
	if _, err := s.repo.GetWorkByID(ctx, workID); err != nil {
		return nil, err
	}

	inputs := make([]domain.FusionInput, 0, len(in.Sources))
	for _, src := range in.Sources {
		character, err := s.repo.GetCharacter(ctx, src.CharacterID)
		if err != nil {
			return nil, err
		}
		if character.CreativeWorkID != workID {
			return nil, errors.New("融合来源必须属于同一个二创作品")
		}
		weight := src.Weight
		if weight <= 0 || weight > 100 {
			return nil, fmt.Errorf("%w：来源整体权重应为 1-100", domain.ErrInheritanceWeightInvalid)
		}
		inputs = append(inputs, domain.FusionInput{
			Character: character.DNA,
			Fusion:    domain.FusionSource{CharacterID: character.ID, Name: character.Name, Weight: weight},
		})
	}

	dna, detail := domain.FuseCharacters(inputs)
	fused := &domain.CreativeCharacter{
		CreativeWorkID: workID,
		Name:           in.Name,
		Description:    in.Description,
		SourceType:     domain.SourceFused,
		DNA:            dna,
		Importance:     in.Importance,
		FusionSources:  make([]domain.FusionSource, 0, len(inputs)),
		FusionDetail:   detail,
	}
	for _, item := range inputs {
		fused.FusionSources = append(fused.FusionSources, item.Fusion)
	}
	fused.Normalize()
	if err := fused.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.CreateCharacter(ctx, fused); err != nil {
		return nil, err
	}

	for _, item := range inputs {
		if err := s.repo.CreateMapping(ctx, &domain.OriginalCreativeMapping{
			CreativeWorkID: workID,
			OriginalType:   "character", OriginalID: sourceIDOf(ctx, s, item.Fusion.CharacterID),
			CreativeType: "creative_character", CreativeID: fused.ID,
			MappingType: domain.MappingFused,
			Description: fmt.Sprintf("融合来源：「%s」（权重 %d）", item.Fusion.Name, item.Fusion.Weight),
		}); err != nil {
			return nil, err
		}
	}
	return fused, nil
}

// sourceIDOf 取二创人物对应的原著人物 ID（没有来源时返回自身 ID，仅用于映射记录）。
func sourceIDOf(ctx context.Context, s *CreativeService, creativeCharacterID string) string {
	character, err := s.repo.GetCharacter(ctx, creativeCharacterID)
	if err != nil || character.SourceCharacterID == nil {
		return creativeCharacterID
	}
	return *character.SourceCharacterID
}

// ListCharacters 列出二创人物。
func (s *CreativeService) ListCharacters(ctx context.Context, workID string) ([]domain.CreativeCharacter, error) {
	if _, err := s.repo.GetWorkByID(ctx, workID); err != nil {
		return nil, err
	}
	items, err := s.repo.ListCharacters(ctx, workID)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []domain.CreativeCharacter{}
	}
	return items, nil
}

// CharacterDetail 是二创人物详情（含继承规则）。
type CharacterDetail struct {
	Character domain.CreativeCharacter
	Rules     []domain.InheritanceRule
}

// GetCharacter 取二创人物详情（含继承权重，便于界面回显"继承了多少"）。
func (s *CreativeService) GetCharacter(ctx context.Context, id string) (*CharacterDetail, error) {
	character, err := s.repo.GetCharacter(ctx, id)
	if err != nil {
		return nil, err
	}
	rules, err := s.repo.ListInheritanceRules(ctx, id)
	if err != nil {
		return nil, err
	}
	if rules == nil {
		rules = []domain.InheritanceRule{}
	}
	return &CharacterDetail{Character: *character, Rules: rules}, nil
}

// SaveCharacter 直接落盘一个人物实体（给版本恢复用：快照回填后整条写回）。
// 不在这里做业务改写，避免"恢复"被当成一次编辑而改变来源类型等语义。
func (s *CreativeService) SaveCharacter(ctx context.Context, c *domain.CreativeCharacter) error {
	if c == nil {
		return domain.ErrCreativeCharacterNotFound
	}
	return s.repo.UpdateCharacter(ctx, c)
}

// WorkIDByCreativeWorld 由二创世界 ID 反查所属二创作品 ID（版本快照要用）。
func (s *CreativeService) WorkIDByCreativeWorld(ctx context.Context, creativeWorldID string) (string, error) {
	return s.repo.GetWorkIDByCreativeWorld(ctx, creativeWorldID)
}

// WorkIDByWorldRule 由二创世界规则 ID 反查所属二创作品 ID。
func (s *CreativeService) WorkIDByWorldRule(ctx context.Context, ruleID string) (string, error) {
	rule, err := s.repo.GetWorldRule(ctx, ruleID)
	if err != nil {
		return "", err
	}
	return s.repo.GetWorkIDByCreativeWorld(ctx, rule.CreativeWorldID)
}

// UpdateCharacterInput 是二创人物更新入参（作者手改 DNA/描述）。
type UpdateCharacterInput struct {
	Name        *string
	Description *string
	DNA         *domain.CharacterDNA
	Importance  *int
	IsLocked    *bool
}

// UpdateCharacter 更新二创人物。
func (s *CreativeService) UpdateCharacter(ctx context.Context, id string, in UpdateCharacterInput) (*domain.CreativeCharacter, error) {
	character, err := s.repo.GetCharacter(ctx, id)
	if err != nil {
		return nil, err
	}
	if in.Name != nil {
		character.Name = *in.Name
		// 作者改名后语义变成"继承并修改"
		if character.SourceType == domain.SourceOriginalInherited {
			character.SourceType = domain.SourceModified
		}
	}
	if in.Description != nil {
		character.Description = *in.Description
	}
	if in.DNA != nil {
		character.DNA = *in.DNA
		if character.SourceType == domain.SourceOriginalInherited {
			character.SourceType = domain.SourceModified
		}
	}
	if in.Importance != nil {
		character.Importance = *in.Importance
	}
	if in.IsLocked != nil {
		character.IsLocked = *in.IsLocked
	}
	character.Normalize()
	if err := character.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateCharacter(ctx, character); err != nil {
		return nil, err
	}
	return character, nil
}

// DeleteCharacter 删除二创人物。
func (s *CreativeService) DeleteCharacter(ctx context.Context, id string) error {
	return s.repo.DeleteCharacter(ctx, id)
}

// ListMappings 列出映射。
func (s *CreativeService) ListMappings(ctx context.Context, workID string) ([]domain.OriginalCreativeMapping, error) {
	if _, err := s.repo.GetWorkByID(ctx, workID); err != nil {
		return nil, err
	}
	items, err := s.repo.ListMappings(ctx, workID)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []domain.OriginalCreativeMapping{}
	}
	return items, nil
}

// DeleteMapping 删除映射。
func (s *CreativeService) DeleteMapping(ctx context.Context, id string) error {
	return s.repo.DeleteMapping(ctx, id)
}

func (s *CreativeService) findByWorkAndName(ctx context.Context, workID, name string) (*domain.CreativeCharacter, error) {
	items, err := s.repo.ListCharacters(ctx, workID)
	if err != nil {
		return nil, err
	}
	for i := range items {
		if strings.EqualFold(items[i].Name, name) {
			return &items[i], nil
		}
	}
	return nil, nil
}

// ---------- 二创世界（规格书 §21-§22） ----------

// WorldDetail 是二创世界详情。
type WorldDetail struct {
	CreativeWorkID string
	World          *domain.CreativeWorld
	Rules          []domain.CreativeWorldRule
}

// InheritWorld 从原著世界继承出一个二创世界。
// FULL / PARTIAL 会把原著规则整套带过来（状态 INHERITED），作者再逐条改；MODIFIED / NEW 先建空世界。
func (s *CreativeService) InheritWorld(ctx context.Context, workID string, mode domain.WorldInheritanceMode) (*WorldDetail, error) {
	work, err := s.repo.GetWorkByID(ctx, workID)
	if err != nil {
		return nil, err
	}
	if !mode.Valid() {
		return nil, fmt.Errorf("%w: %s", domain.ErrCreativeWorldModeBad, mode)
	}

	var sourceWorld *domain.OriginalWorld
	if s.origWorlds != nil {
		if w, err := s.origWorlds.GetWorld(ctx, work.OriginalWorkID); err == nil {
			sourceWorld = w
		} else if !errors.Is(err, domain.ErrWorldNotFound) {
			return nil, err
		}
	}

	world := &domain.CreativeWorld{CreativeWorkID: workID, InheritanceMode: mode}
	if sourceWorld != nil {
		world.SourceWorldID = &sourceWorld.ID
		world.Name = sourceWorld.Name
		world.Description = sourceWorld.Description
	}
	world.Normalize()
	if err := world.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.UpsertWorld(ctx, world); err != nil {
		return nil, err
	}

	if mode.CopiesRules() && sourceWorld != nil && s.origWorlds != nil {
		rules, err := s.origWorlds.ListRules(ctx, sourceWorld.ID)
		if err != nil {
			return nil, err
		}
		for _, src := range rules {
			rule := &domain.CreativeWorldRule{
				CreativeWorldID: world.ID, SourceRuleID: &src.ID, Status: domain.RuleInherited,
				Category: src.Category, Name: src.Name, Description: src.Description, Importance: src.Importance,
			}
			rule.Normalize()
			if err := rule.Validate(); err != nil {
				continue // 原著规则不合法就跳过（不阻断整体继承）
			}
			if err := s.repo.CreateWorldRule(ctx, rule); err != nil {
				if errors.Is(err, domain.ErrCreativeWorldRuleDup) {
					continue
				}
				return nil, err
			}
		}
		if err := s.repo.CreateMapping(ctx, &domain.OriginalCreativeMapping{
			CreativeWorkID: workID,
			OriginalType:   "world", OriginalID: sourceWorld.ID,
			CreativeType: "creative_world", CreativeID: world.ID,
			MappingType: domain.MappingInherited,
			Description: fmt.Sprintf("按 %s 模式继承原著世界「%s」", mode, sourceWorld.Name),
		}); err != nil {
			return nil, err
		}
	}
	return s.GetWorldDetail(ctx, workID)
}

// GetWorldDetail 取二创世界（含规则）；还没建世界时返回 (nil, nil)。
func (s *CreativeService) GetWorldDetail(ctx context.Context, workID string) (*WorldDetail, error) {
	// 先确认二创作品存在：否则"作品不存在"和"还没建世界"都会返回空壳，调用方无法区分
	if _, err := s.repo.GetWorkByID(ctx, workID); err != nil {
		return nil, err
	}
	world, err := s.repo.GetWorld(ctx, workID)
	if err != nil {
		if errors.Is(err, domain.ErrCreativeWorldNotFound) {
			return &WorldDetail{CreativeWorkID: workID, Rules: []domain.CreativeWorldRule{}}, nil
		}
		return nil, err
	}
	rules, err := s.repo.ListWorldRules(ctx, world.ID)
	if err != nil {
		return nil, err
	}
	if rules == nil {
		rules = []domain.CreativeWorldRule{}
	}
	return &WorldDetail{CreativeWorkID: workID, World: world, Rules: rules}, nil
}

// UpdateWorldInput 是二创世界更新入参。
type UpdateWorldInput struct {
	Name            *string
	Description     *string
	InheritanceMode *domain.WorldInheritanceMode
}

// UpdateWorld 更新二创世界。
func (s *CreativeService) UpdateWorld(ctx context.Context, workID string, in UpdateWorldInput) (*WorldDetail, error) {
	world, err := s.repo.GetWorld(ctx, workID)
	if err != nil {
		return nil, err
	}
	if in.Name != nil {
		world.Name = *in.Name
	}
	if in.Description != nil {
		world.Description = *in.Description
	}
	if in.InheritanceMode != nil {
		world.InheritanceMode = *in.InheritanceMode
	}
	world.Normalize()
	if err := world.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.UpsertWorld(ctx, world); err != nil {
		return nil, err
	}
	return s.GetWorldDetail(ctx, workID)
}

// WorldRuleInput 是二创世界规则入参。
type WorldRuleInput struct {
	Category     string
	Name         string
	Description  string
	Importance   int
	Status       domain.CreativeWorldRuleStatus
	SourceRuleID *string
}

// CreateWorldRule 新增二创世界规则（默认 NEW；带 source_rule_id 表示从原著规则改出来）。
func (s *CreativeService) CreateWorldRule(ctx context.Context, workID string, in WorldRuleInput) (*domain.CreativeWorldRule, error) {
	world, err := s.repo.GetWorld(ctx, workID)
	if err != nil {
		return nil, err
	}
	status := in.Status
	if status == "" {
		if in.SourceRuleID != nil {
			status = domain.RuleModified
		} else {
			status = domain.RuleNew
		}
	}
	rule := &domain.CreativeWorldRule{
		CreativeWorldID: world.ID, SourceRuleID: in.SourceRuleID, Status: status,
		Category: in.Category, Name: in.Name, Description: in.Description, Importance: in.Importance,
	}
	rule.Normalize()
	if err := rule.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.CreateWorldRule(ctx, rule); err != nil {
		return nil, err
	}
	return rule, nil
}

// UpdateWorldRule 更新二创世界规则。
func (s *CreativeService) UpdateWorldRule(ctx context.Context, id string, in WorldRuleInput) (*domain.CreativeWorldRule, error) {
	rule, err := s.repo.GetWorldRule(ctx, id)
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
	if in.Status != "" {
		rule.Status = in.Status
	} else if rule.Status == domain.RuleInherited {
		rule.Status = domain.RuleModified
	}
	rule.Normalize()
	if err := rule.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateWorldRule(ctx, rule); err != nil {
		return nil, err
	}
	return rule, nil
}

// RemoveWorldRule 在二创里"删掉"一条规则：
// 继承来的规则标记为 REMOVED（保留可追溯性），纯新增的规则直接软删除。
func (s *CreativeService) RemoveWorldRule(ctx context.Context, id string) error {
	rule, err := s.repo.GetWorldRule(ctx, id)
	if err != nil {
		return err
	}
	if rule.SourceRuleID != nil {
		rule.Status = domain.RuleRemoved
		if err := s.repo.UpdateWorldRule(ctx, rule); err != nil {
			return err
		}
		workID, err := s.repo.GetWorkIDByCreativeWorld(ctx, rule.CreativeWorldID)
		if err != nil {
			return err
		}
		return s.repo.CreateMapping(ctx, &domain.OriginalCreativeMapping{
			CreativeWorkID: workID,
			OriginalType:   "world_rule", OriginalID: *rule.SourceRuleID,
			CreativeType: "creative_world_rule", CreativeID: rule.ID,
			MappingType: domain.MappingRemoved,
			Description: "在二创中删除了这条原著规则",
		})
	}
	return s.repo.DeleteWorldRule(ctx, id)
}

// ---------- 分叉点与二创时间线（规格书 §24-§25） ----------

// DivergenceInput 是分叉点入参。
type DivergenceInput struct {
	OriginalChapterID *string
	OriginalEventID   *string
	TimeLabel         string
	Description       string
}

// SetDivergence 设置分叉点；会校验指向的事件/章节属于该二创作品依据的原著。
func (s *CreativeService) SetDivergence(ctx context.Context, workID string, in DivergenceInput) (*domain.DivergencePoint, error) {
	work, err := s.repo.GetWorkByID(ctx, workID)
	if err != nil {
		return nil, err
	}
	if in.OriginalEventID != nil && *in.OriginalEventID != "" {
		if s.origTimelines == nil {
			return nil, domain.ErrDivergenceSourceInvalid
		}
		event, err := s.origTimelines.GetEvent(ctx, *in.OriginalEventID)
		if err != nil {
			if errors.Is(err, domain.ErrEventNotFound) {
				return nil, domain.ErrDivergenceSourceInvalid
			}
			return nil, err
		}
		if event.OriginalWorkID != work.OriginalWorkID {
			return nil, domain.ErrDivergenceSourceInvalid
		}
	}
	point := &domain.DivergencePoint{
		CreativeWorkID:    workID,
		OriginalChapterID: in.OriginalChapterID, OriginalEventID: in.OriginalEventID,
		TimeLabel: in.TimeLabel, Description: in.Description,
	}
	point.Normalize()
	if err := point.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.UpsertDivergence(ctx, point); err != nil {
		return nil, err
	}
	if err := s.repo.SetDivergencePoint(ctx, workID, point.ID); err != nil {
		return nil, err
	}
	return point, nil
}

// GetDivergence 取分叉点；没设过返回 (nil, nil)。
func (s *CreativeService) GetDivergence(ctx context.Context, workID string) (*domain.DivergencePoint, error) {
	point, err := s.repo.GetDivergence(ctx, workID)
	if err != nil {
		if errors.Is(err, domain.ErrDivergenceNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return point, nil
}

// BuildTimeline 自动构建二创时间线：把分叉点（含）之前的原著事件按序继承过来，并保留作者已有的二创新事件。
func (s *CreativeService) BuildTimeline(ctx context.Context, workID string) ([]domain.CreativeTimelineEvent, error) {
	work, err := s.repo.GetWorkByID(ctx, workID)
	if err != nil {
		return nil, err
	}
	existing, err := s.repo.ListTimeline(ctx, workID)
	if err != nil {
		return nil, err
	}
	// 作者自己加的条目（没有原著来源）保持在末尾
	creativeOnly := make([]domain.CreativeTimelineEvent, 0)
	for _, e := range existing {
		if e.SourceOriginalEventID == nil && e.Status != domain.TimelineRemoved {
			creativeOnly = append(creativeOnly, e)
		}
	}

	inherited := make([]domain.CreativeTimelineEvent, 0)
	if s.origTimelines != nil {
		timeline, err := s.origTimelines.GetTimeline(ctx, work.OriginalWorkID)
		if err == nil {
			entries, err := s.origTimelines.ListTimelineEntries(ctx, timeline.ID)
			if err != nil {
				return nil, err
			}
			cutoff := -1
			if point, _ := s.GetDivergence(ctx, workID); point != nil && point.OriginalEventID != nil {
				for i, e := range entries {
					if e.EventID == *point.OriginalEventID {
						cutoff = i
						break
					}
				}
			}
			for i, entry := range entries {
				if cutoff >= 0 && i > cutoff {
					break // 分叉点之后不再继承
				}
				event, err := s.origTimelines.GetEvent(ctx, entry.EventID)
				if err != nil {
					continue
				}
				eventID := event.ID
				inherited = append(inherited, domain.CreativeTimelineEvent{
					CreativeWorkID: workID, SourceOriginalEventID: &eventID,
					Status: domain.TimelineInherited, TimeLabel: entry.TimeLabel,
					Title: event.Title, Description: event.Description,
				})
			}
		} else if !errors.Is(err, domain.ErrTimelineNotFound) {
			return nil, err
		}
	}

	final := append(inherited, creativeOnly...)
	for i := range final {
		final[i].Normalize()
		if err := final[i].Validate(); err != nil {
			return nil, err
		}
	}
	if err := s.repo.ReplaceTimeline(ctx, workID, final); err != nil {
		return nil, err
	}
	// 继承过来的事件记映射。
	// 注意：entry 的 ID 是 ReplaceTimeline 内部生成的，必须回读拿到真实 ID，
	// 否则会写出空 creative_id（冒烟测试抓到过）。
	stored, err := s.repo.ListTimeline(ctx, workID)
	if err != nil {
		return nil, err
	}
	storedBySource := map[string]string{}
	for _, e := range stored {
		if e.SourceOriginalEventID != nil {
			storedBySource[*e.SourceOriginalEventID] = e.ID
		}
	}
	for _, e := range inherited {
		if e.SourceOriginalEventID == nil {
			continue
		}
		creativeID, ok := storedBySource[*e.SourceOriginalEventID]
		if !ok {
			continue
		}
		if err := s.repo.CreateMapping(ctx, &domain.OriginalCreativeMapping{
			CreativeWorkID: workID,
			OriginalType:   "event", OriginalID: *e.SourceOriginalEventID,
			CreativeType: "creative_timeline_event", CreativeID: creativeID,
			MappingType: domain.MappingInherited,
			Description: "分叉点之前的原著事件，按序继承",
		}); err != nil {
			return nil, err
		}
	}
	return stored, nil
}

// CreativeTimelineItemInput 是手工编排二创时间线的一项。
type CreativeTimelineItemInput struct {
	SourceOriginalEventID *string
	Status                domain.CreativeTimelineEventStatus
	TimeLabel             string
	Title                 string
	Description           string
}

// SetTimeline 手工整体替换二创时间线（幂等）。
func (s *CreativeService) SetTimeline(ctx context.Context, workID string, items []CreativeTimelineItemInput) ([]domain.CreativeTimelineEvent, error) {
	if _, err := s.repo.GetWorkByID(ctx, workID); err != nil {
		return nil, err
	}
	events := make([]domain.CreativeTimelineEvent, 0, len(items))
	for _, in := range items {
		e := domain.CreativeTimelineEvent{
			CreativeWorkID: workID, SourceOriginalEventID: in.SourceOriginalEventID,
			Status: in.Status, TimeLabel: in.TimeLabel, Title: in.Title, Description: in.Description,
		}
		e.Normalize()
		if err := e.Validate(); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	if err := s.repo.ReplaceTimeline(ctx, workID, events); err != nil {
		return nil, err
	}
	return s.repo.ListTimeline(ctx, workID)
}

// GetTimeline 取二创时间线。
func (s *CreativeService) GetTimeline(ctx context.Context, workID string) ([]domain.CreativeTimelineEvent, error) {
	if _, err := s.repo.GetWorkByID(ctx, workID); err != nil {
		return nil, err
	}
	items, err := s.repo.ListTimeline(ctx, workID)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []domain.CreativeTimelineEvent{}
	}
	return items, nil
}
