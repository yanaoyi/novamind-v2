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
}

// CharacterLookup 用于读取原著人物（继承来源）。
type CharacterSourceLookup interface {
	GetCharacter(ctx context.Context, id string) (*domain.OriginalCharacter, error)
}

// CreativeService 是二创作品的业务服务。
type CreativeService struct {
	repo       CreativeRepository
	projects   ProjectRepository
	works      WorkLookup
	characters CharacterSourceLookup
}

// NewCreativeService 构建服务。
func NewCreativeService(
	repo CreativeRepository,
	projects ProjectRepository,
	works WorkLookup,
	characters CharacterSourceLookup,
) *CreativeService {
	return &CreativeService{repo: repo, projects: projects, works: works, characters: characters}
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
