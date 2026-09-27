package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// 人物与关系相关错误。
var (
	ErrCharacterNameRequired = errors.New("人物姓名不能为空")
	ErrCharacterNameTooLong  = errors.New("人物姓名不能超过 120 个字符")
	ErrCharacterNotFound     = errors.New("人物不存在")
	ErrCharacterDuplicate    = errors.New("同名人物已存在")
	ErrCharacterImportance   = errors.New("重要度必须是 1-5")
	ErrDNAWeightOutOfRange   = errors.New("人物 DNA 权重必须在 0-100 之间")
	ErrRelationTypeInvalid   = errors.New("关系类型非法")
	ErrRelationSelfReference = errors.New("人物不能与自己建立关系")
	ErrRelationStrength      = errors.New("关系强度必须在 0-100 之间")
	ErrRelationNotFound      = errors.New("人物关系不存在")
	ErrRelationDuplicate     = errors.New("同样的关系已存在")
	ErrRelationCrossWork     = errors.New("关系两端人物必须属于同一部原著")
)

// CharacterSource 标识数据来源（Phase 2 手工录入，Phase 3 AI 提取）。
type CharacterSource string

const (
	SourceManual CharacterSource = "MANUAL"
	SourceAI     CharacterSource = "AI"
)

// Valid 判断来源是否合法。
func (s CharacterSource) Valid() bool { return s == SourceManual || s == SourceAI }

// RelationType 是人物关系类型（规格书 §12）。
type RelationType string

const (
	RelationFamily       RelationType = "family"
	RelationFriend       RelationType = "friend"
	RelationLover        RelationType = "lover"
	RelationEnemy        RelationType = "enemy"
	RelationMentor       RelationType = "mentor"
	RelationStudent      RelationType = "student"
	RelationColleague    RelationType = "colleague"
	RelationRival        RelationType = "rival"
	RelationOrganization RelationType = "organization"
	RelationOther        RelationType = "other"
)

// Valid 判断关系类型是否合法。
func (t RelationType) Valid() bool {
	switch t {
	case RelationFamily, RelationFriend, RelationLover, RelationEnemy, RelationMentor,
		RelationStudent, RelationColleague, RelationRival, RelationOrganization, RelationOther:
		return true
	default:
		return false
	}
}

// DNADimension 是人物 DNA 的单个维度：一句描述 + 0-100 的权重。
// 权重表达"继承该维度的程度"，是后续二创继承（§19 InheritanceRule）的基础。
type DNADimension struct {
	Text   string `json:"text"`
	Weight int    `json:"weight"`
}

// CharacterDNA 是人物 DNA（规格书 §11）。
// 维度与规格书一一对应；零值表示该维度未填写。
type CharacterDNA struct {
	Personality         DNADimension `json:"personality"`
	Values              DNADimension `json:"values"`
	Motivation          DNADimension `json:"motivation"`
	Behavior            DNADimension `json:"behavior"`
	SpeechStyle         DNADimension `json:"speech_style"`
	Background          DNADimension `json:"background"`
	Ability             DNADimension `json:"ability"`
	DecisionStyle       DNADimension `json:"decision_style"`
	ConflictResponse    DNADimension `json:"conflict_response"`
	EmotionalResponse   DNADimension `json:"emotional_response"`
	RelationshipPattern DNADimension `json:"relationship_pattern"`
}

// Dimensions 返回全部维度（键为 JSON 字段名），便于统一校验与遍历。
func (d CharacterDNA) Dimensions() map[string]DNADimension {
	return map[string]DNADimension{
		"personality":          d.Personality,
		"values":               d.Values,
		"motivation":           d.Motivation,
		"behavior":             d.Behavior,
		"speech_style":         d.SpeechStyle,
		"background":           d.Background,
		"ability":              d.Ability,
		"decision_style":       d.DecisionStyle,
		"conflict_response":    d.ConflictResponse,
		"emotional_response":   d.EmotionalResponse,
		"relationship_pattern": d.RelationshipPattern,
	}
}

// Validate 校验每个维度的权重范围。
func (d CharacterDNA) Validate() error {
	for name, dim := range d.Dimensions() {
		if dim.Weight < 0 || dim.Weight > 100 {
			return fmt.Errorf("%w：%s=%d", ErrDNAWeightOutOfRange, name, dim.Weight)
		}
	}
	return nil
}

// OriginalCharacter 是原著人物（规格书 §10）。
type OriginalCharacter struct {
	ID               string
	OriginalWorkID   string
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
	DNA              CharacterDNA
	Importance       int
	Source           CharacterSource
	Notes            string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	DeletedAt        *time.Time
}

// Validate 校验人物实体。
func (c *OriginalCharacter) Validate() error {
	name := strings.TrimSpace(c.Name)
	if name == "" {
		return ErrCharacterNameRequired
	}
	if len([]rune(name)) > 120 {
		return ErrCharacterNameTooLong
	}
	if c.Importance != 0 && (c.Importance < 1 || c.Importance > 5) {
		return ErrCharacterImportance
	}
	if c.Source != "" && !c.Source.Valid() {
		return fmt.Errorf("人物来源非法: %s", c.Source)
	}
	return c.DNA.Validate()
}

// Normalize 清洗输入并补默认值。
func (c *OriginalCharacter) Normalize() {
	c.Name = strings.TrimSpace(c.Name)
	c.Role = strings.TrimSpace(c.Role)
	c.Gender = strings.TrimSpace(c.Gender)
	c.Age = strings.TrimSpace(c.Age)
	if c.Importance == 0 {
		c.Importance = 3
	}
	if c.Source == "" {
		c.Source = SourceManual
	}
	// 别名去空白、去重
	seen := map[string]bool{}
	aliases := make([]string, 0, len(c.Aliases))
	for _, a := range c.Aliases {
		a = strings.TrimSpace(a)
		if a == "" || seen[a] {
			continue
		}
		seen[a] = true
		aliases = append(aliases, a)
	}
	c.Aliases = aliases
}

// CharacterRelationship 是人物关系（有向边，规格书 §12）。
type CharacterRelationship struct {
	ID                string
	OriginalWorkID    string
	SourceCharacterID string
	TargetCharacterID string
	RelationType      RelationType
	Strength          int
	Description       string
	Source            CharacterSource
	CreatedAt         time.Time
	UpdatedAt         time.Time
	DeletedAt         *time.Time
}

// Validate 校验关系实体。
func (r *CharacterRelationship) Validate() error {
	if r.SourceCharacterID == "" || r.TargetCharacterID == "" {
		return errors.New("关系两端的人物 ID 不能为空")
	}
	if r.SourceCharacterID == r.TargetCharacterID {
		return ErrRelationSelfReference
	}
	if !r.RelationType.Valid() {
		return ErrRelationTypeInvalid
	}
	if r.Strength < 0 || r.Strength > 100 {
		return ErrRelationStrength
	}
	return nil
}

// Normalize 补默认值。
func (r *CharacterRelationship) Normalize() {
	if r.Strength == 0 {
		r.Strength = 50
	}
	if r.Source == "" {
		r.Source = SourceManual
	}
}
