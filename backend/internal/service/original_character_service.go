package service

import (
	"context"
	"strings"

	"github.com/yanaoyi/novamindv2/backend/internal/domain"
	"github.com/yanaoyi/novamindv2/backend/internal/repository"
)

// OriginalCharacterRepository 是人物 service 需要的仓储能力。
type OriginalCharacterRepository interface {
	CreateCharacter(ctx context.Context, c *domain.OriginalCharacter) error
	GetCharacter(ctx context.Context, id string) (*domain.OriginalCharacter, error)
	ListCharacters(ctx context.Context, workID string, f repository.CharacterFilter) ([]domain.OriginalCharacter, int64, error)
	UpdateCharacter(ctx context.Context, c *domain.OriginalCharacter) error
	DeleteCharacter(ctx context.Context, id string) error

	CreateRelationship(ctx context.Context, r *domain.CharacterRelationship) error
	ListRelationships(ctx context.Context, workID string) ([]domain.CharacterRelationship, error)
	GetRelationship(ctx context.Context, id string) (*domain.CharacterRelationship, error)
	UpdateRelationship(ctx context.Context, r *domain.CharacterRelationship) error
	DeleteRelationship(ctx context.Context, id string) error
}

// WorkLookup 是"确认原著存在"所需的最小能力（避免 service 依赖整个 OriginalRepository）。
type WorkLookup interface {
	GetWorkByID(ctx context.Context, id string) (*domain.OriginalWork, error)
}

// OriginalCharacterService 是原著人物与人物关系的业务服务。
type OriginalCharacterService struct {
	repo  OriginalCharacterRepository
	works WorkLookup
}

// NewOriginalCharacterService 构建服务。
func NewOriginalCharacterService(repo OriginalCharacterRepository, works WorkLookup) *OriginalCharacterService {
	return &OriginalCharacterService{repo: repo, works: works}
}

// CharacterInput 是人物创建/更新入参（全量覆盖式更新）。
type CharacterInput struct {
	Name             string
	Aliases          []string
	Role             string
	Gender           string
	Age              string
	Appearance       string
	Personality      string
	Motivation       string
	Values           string
	Fears            string
	Desires          string
	BehaviorPatterns string
	SpeechStyle      string
	Abilities        string
	FirstAppearance  string
	LastAppearance   string
	DNA              domain.CharacterDNA
	Importance       int
	Notes            string
}

func (in CharacterInput) apply(c *domain.OriginalCharacter) {
	c.Name = in.Name
	c.Aliases = in.Aliases
	c.Role = in.Role
	c.Gender = in.Gender
	c.Age = in.Age
	c.Appearance = in.Appearance
	c.Personality = in.Personality
	c.Motivation = in.Motivation
	c.Values = in.Values
	c.Fears = in.Fears
	c.Desires = in.Desires
	c.BehaviorPatterns = in.BehaviorPatterns
	c.SpeechStyle = in.SpeechStyle
	c.Abilities = in.Abilities
	c.FirstAppearance = in.FirstAppearance
	c.LastAppearance = in.LastAppearance
	c.DNA = in.DNA
	c.Importance = in.Importance
	c.Notes = in.Notes
}

// CreateCharacter 新增人物。
func (s *OriginalCharacterService) CreateCharacter(ctx context.Context, workID string, in CharacterInput) (*domain.OriginalCharacter, error) {
	if _, err := s.works.GetWorkByID(ctx, workID); err != nil {
		return nil, err
	}
	c := &domain.OriginalCharacter{OriginalWorkID: workID}
	in.apply(c)
	c.Normalize()
	if err := c.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.CreateCharacter(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

// GetCharacter 取人物详情。
func (s *OriginalCharacterService) GetCharacter(ctx context.Context, id string) (*domain.OriginalCharacter, error) {
	if strings.TrimSpace(id) == "" {
		return nil, domain.ErrCharacterNotFound
	}
	return s.repo.GetCharacter(ctx, id)
}

// ListCharacters 列出某原著人物。
func (s *OriginalCharacterService) ListCharacters(ctx context.Context, workID string, page, pageSize int, keyword string) ([]domain.OriginalCharacter, int64, error) {
	if _, err := s.works.GetWorkByID(ctx, workID); err != nil {
		return nil, 0, err
	}
	items, total, err := s.repo.ListCharacters(ctx, workID, repository.CharacterFilter{
		Keyword:  strings.TrimSpace(keyword),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		return nil, 0, err
	}
	if items == nil {
		items = []domain.OriginalCharacter{}
	}
	return items, total, nil
}

// UpdateCharacter 全量更新人物。
func (s *OriginalCharacterService) UpdateCharacter(ctx context.Context, id string, in CharacterInput) (*domain.OriginalCharacter, error) {
	c, err := s.repo.GetCharacter(ctx, id)
	if err != nil {
		return nil, err
	}
	in.apply(c)
	c.Normalize()
	if err := c.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateCharacter(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

// DeleteCharacter 删除人物（连带其关系）。
func (s *OriginalCharacterService) DeleteCharacter(ctx context.Context, id string) error {
	return s.repo.DeleteCharacter(ctx, id)
}

// RelationshipInput 是关系创建/更新入参。
type RelationshipInput struct {
	SourceCharacterID string
	TargetCharacterID string
	RelationType      domain.RelationType
	Strength          int
	Description       string
}

// CreateRelationship 新增人物关系。
// 约束：两端人物必须存在，且属于同一部原著。
func (s *OriginalCharacterService) CreateRelationship(ctx context.Context, workID string, in RelationshipInput) (*domain.CharacterRelationship, error) {
	if _, err := s.works.GetWorkByID(ctx, workID); err != nil {
		return nil, err
	}
	source, err := s.repo.GetCharacter(ctx, in.SourceCharacterID)
	if err != nil {
		return nil, err
	}
	target, err := s.repo.GetCharacter(ctx, in.TargetCharacterID)
	if err != nil {
		return nil, err
	}
	if source.OriginalWorkID != workID || target.OriginalWorkID != workID {
		return nil, domain.ErrRelationCrossWork
	}

	rel := &domain.CharacterRelationship{
		OriginalWorkID:    workID,
		SourceCharacterID: in.SourceCharacterID,
		TargetCharacterID: in.TargetCharacterID,
		RelationType:      in.RelationType,
		Strength:          in.Strength,
		Description:       strings.TrimSpace(in.Description),
	}
	rel.Normalize()
	if err := rel.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.CreateRelationship(ctx, rel); err != nil {
		return nil, err
	}
	return rel, nil
}

// ListRelationships 列出某原著全部关系。
func (s *OriginalCharacterService) ListRelationships(ctx context.Context, workID string) ([]domain.CharacterRelationship, error) {
	if _, err := s.works.GetWorkByID(ctx, workID); err != nil {
		return nil, err
	}
	items, err := s.repo.ListRelationships(ctx, workID)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []domain.CharacterRelationship{}
	}
	return items, nil
}

// UpdateRelationship 更新关系（只允许改类型/强度/描述）。
func (s *OriginalCharacterService) UpdateRelationship(ctx context.Context, id string, in RelationshipInput) (*domain.CharacterRelationship, error) {
	rel, err := s.repo.GetRelationship(ctx, id)
	if err != nil {
		return nil, err
	}
	if in.RelationType != "" {
		rel.RelationType = in.RelationType
	}
	if in.Strength > 0 {
		rel.Strength = in.Strength
	}
	rel.Description = strings.TrimSpace(in.Description)
	if err := rel.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateRelationship(ctx, rel); err != nil {
		return nil, err
	}
	return rel, nil
}

// DeleteRelationship 删除关系。
func (s *OriginalCharacterService) DeleteRelationship(ctx context.Context, id string) error {
	return s.repo.DeleteRelationship(ctx, id)
}
