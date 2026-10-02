package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// 二创相关错误。
var (
	ErrCreativeTitleEmpty        = errors.New("二创作品标题不能为空")
	ErrCreativeNotFound          = errors.New("二创作品不存在")
	ErrCreativeAlreadyExists     = errors.New("该工程已存在二创作品")
	ErrCreativeNotCreativeProj   = errors.New("只有 CREATIVE 类型的工程可以创建二创作品")
	ErrCreativeCharacterNotFound = errors.New("二创人物不存在")
	ErrCreativeCharacterName     = errors.New("二创人物姓名不能为空")
	ErrCreativeCharacterDup      = errors.New("同名二创人物已存在")
	ErrCreativeCharacterLocked   = errors.New("该人物已锁定，不能覆盖继承结果")
	ErrSourceCharacterNotFound   = errors.New("原著人物不存在")
	ErrSourceCharacterCrossWork  = errors.New("原著人物不属于该二创作品所依据的原著")
	ErrInheritanceWeightInvalid  = errors.New("继承权重必须在 0-100 之间")
	ErrInheritanceNoDimension    = errors.New("继承权重至少要有一个维度大于 0")
	ErrFusionNeedsTwoSources     = errors.New("人物融合至少需要两个来源")
	ErrMappingNotFound           = errors.New("映射关系不存在")
)

// CreativeWorkStatus 是二创作品状态。
type CreativeWorkStatus string

const (
	CreativeDraft    CreativeWorkStatus = "DRAFT"
	CreativeWriting  CreativeWorkStatus = "WRITING"
	CreativeFinished CreativeWorkStatus = "FINISHED"
)

// Valid 判断状态是否合法。
func (s CreativeWorkStatus) Valid() bool {
	return s == CreativeDraft || s == CreativeWriting || s == CreativeFinished
}

// CreativeSourceType 是二创人物的来源类型（SPEC.md §9.2）。
type CreativeSourceType string

const (
	SourceOriginalInherited CreativeSourceType = "ORIGINAL_INHERITED"
	SourceModified          CreativeSourceType = "MODIFIED"
	SourceFused             CreativeSourceType = "FUSED"
	SourceNew               CreativeSourceType = "NEW"
)

// Valid 判断来源类型是否合法。
func (t CreativeSourceType) Valid() bool {
	switch t {
	case SourceOriginalInherited, SourceModified, SourceFused, SourceNew:
		return true
	default:
		return false
	}
}

// MappingType 是映射类型（SPEC.md §9.5）。
type MappingType string

const (
	MappingInherited MappingType = "INHERITED"
	MappingModified  MappingType = "MODIFIED"
	MappingReplaced  MappingType = "REPLACED"
	MappingFused     MappingType = "FUSED"
	MappingRemoved   MappingType = "REMOVED"
	MappingNew       MappingType = "NEW"
)

// Valid 判断映射类型是否合法。
func (m MappingType) Valid() bool {
	switch m {
	case MappingInherited, MappingModified, MappingReplaced, MappingFused, MappingRemoved, MappingNew:
		return true
	default:
		return false
	}
}

// CreativeWork 是二创作品（SPEC.md §9.1）。
type CreativeWork struct {
	ID                string
	ProjectID         string
	OriginalWorkID    string
	Title             string
	Description       string
	Status            CreativeWorkStatus
	DivergencePointID *string
	CreatedAt         time.Time
	UpdatedAt         time.Time
	DeletedAt         *time.Time
}

// Normalize 清洗输入并补默认值。
func (w *CreativeWork) Normalize() {
	w.Title = strings.TrimSpace(w.Title)
	w.Description = strings.TrimSpace(w.Description)
	if w.Status == "" {
		w.Status = CreativeDraft
	}
}

// Validate 校验二创作品。
func (w *CreativeWork) Validate() error {
	if strings.TrimSpace(w.Title) == "" {
		return ErrCreativeTitleEmpty
	}
	if !w.Status.Valid() {
		return fmt.Errorf("二创作品状态非法: %s", w.Status)
	}
	return nil
}

// InheritanceRule 是人物继承权重（SPEC.md §9.3）。
// 每个维度 0-100：0 表示不继承该维度，100 表示完全保留。
type InheritanceRule struct {
	ID                  string
	CreativeCharacterID string
	SourceCharacterID   string
	PersonalityWeight   int
	ValueWeight         int
	MotivationWeight    int
	BehaviorWeight      int
	SpeechWeight        int
	BackgroundWeight    int
	AbilityWeight       int
	RelationshipWeight  int
	CreatedAt           time.Time
	UpdatedAt           time.Time
	DeletedAt           *time.Time
}

// Weights 返回各维度权重（键为 DNA 维度名）。
func (r InheritanceRule) Weights() map[string]int {
	return map[string]int{
		"personality":          r.PersonalityWeight,
		"values":               r.ValueWeight,
		"motivation":           r.MotivationWeight,
		"behavior":             r.BehaviorWeight,
		"speech_style":         r.SpeechWeight,
		"background":           r.BackgroundWeight,
		"ability":              r.AbilityWeight,
		"relationship_pattern": r.RelationshipWeight,
	}
}

// Validate 校验权重范围与"至少继承一个维度"。
func (r InheritanceRule) Validate() error {
	positive := 0
	for name, w := range r.Weights() {
		if w < 0 || w > 100 {
			return fmt.Errorf("%w：%s=%d", ErrInheritanceWeightInvalid, name, w)
		}
		if w > 0 {
			positive++
		}
	}
	if positive == 0 {
		return ErrInheritanceNoDimension
	}
	return nil
}

// FusionSource 是融合的一个来源（SPEC.md §9.4）。
type FusionSource struct {
	CharacterID string `json:"character_id"`
	Name        string `json:"name"`
	Weight      int    `json:"weight"` // 该来源整体贡献 0-100
}

// FusionAttribution 记录某个 DNA 维度最终取自哪个来源。
type FusionAttribution struct {
	Dimension string `json:"dimension"`
	FromName  string `json:"from_name"`
	FromID    string `json:"from_id"`
	Weight    int    `json:"weight"`
}

// CreativeCharacter 是二创人物（SPEC.md §9.2）。
type CreativeCharacter struct {
	ID                string
	CreativeWorkID    string
	Name              string
	Description       string
	SourceType        CreativeSourceType
	SourceCharacterID *string
	DNA               CharacterDNA
	FusionSources     []FusionSource
	FusionDetail      []FusionAttribution
	Modifications     map[string]any
	Importance        int
	IsLocked          bool
	CreatedAt         time.Time
	UpdatedAt         time.Time
	DeletedAt         *time.Time
}

// Normalize 清洗输入并补默认值。
func (c *CreativeCharacter) Normalize() {
	c.Name = strings.TrimSpace(c.Name)
	c.Description = strings.TrimSpace(c.Description)
	if c.Importance == 0 {
		c.Importance = 3
	}
	if c.Modifications == nil {
		c.Modifications = map[string]any{}
	}
	if c.FusionSources == nil {
		c.FusionSources = []FusionSource{}
	}
	if c.FusionDetail == nil {
		c.FusionDetail = []FusionAttribution{}
	}
}

// Validate 校验二创人物。
func (c *CreativeCharacter) Validate() error {
	if strings.TrimSpace(c.Name) == "" {
		return ErrCreativeCharacterName
	}
	if !c.SourceType.Valid() {
		return fmt.Errorf("二创人物来源类型非法: %s", c.SourceType)
	}
	// 与原著人物保持一致：0 表示"未设置"（Normalize 会补 3），只拒绝越界值
	if c.Importance != 0 && (c.Importance < 1 || c.Importance > 5) {
		return ErrCharacterImportance
	}
	return c.DNA.Validate()
}

// OriginalCreativeMapping 是原著↔二创映射（SPEC.md §9.5）。
type OriginalCreativeMapping struct {
	ID             string
	CreativeWorkID string
	OriginalType   string
	OriginalID     string
	CreativeType   string
	CreativeID     string
	MappingType    MappingType
	Description    string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      *time.Time
}

// Validate 校验映射。
func (m *OriginalCreativeMapping) Validate() error {
	if m.CreativeWorkID == "" || m.OriginalID == "" || m.CreativeID == "" {
		return errors.New("映射的工程/原著/二创 ID 不能为空")
	}
	if !m.MappingType.Valid() {
		return fmt.Errorf("映射类型非法: %s", m.MappingType)
	}
	return nil
}

// ApplyInheritance 按继承权重从原著人物 DNA 派生出二创人物 DNA。
//
// 规则：新权重 = 原著该维度权重 × 继承权重 ÷ 100；继承权重为 0 的维度直接丢弃。
// 这就是SPEC.md §9.3 示例（性格 90%、语言风格 30%、能力 0%）的落地算法。
func ApplyInheritance(source CharacterDNA, rule InheritanceRule) CharacterDNA {
	scale := func(d DNADimension, w int) DNADimension {
		if w <= 0 {
			return DNADimension{}
		}
		return DNADimension{Text: d.Text, Weight: d.Weight * w / 100}
	}
	return CharacterDNA{
		Personality:         scale(source.Personality, rule.PersonalityWeight),
		Values:              scale(source.Values, rule.ValueWeight),
		Motivation:          scale(source.Motivation, rule.MotivationWeight),
		Behavior:            scale(source.Behavior, rule.BehaviorWeight),
		SpeechStyle:         scale(source.SpeechStyle, rule.SpeechWeight),
		Background:          scale(source.Background, rule.BackgroundWeight),
		Ability:             scale(source.Ability, rule.AbilityWeight),
		DecisionStyle:       scale(source.DecisionStyle, rule.PersonalityWeight),
		ConflictResponse:    scale(source.ConflictResponse, rule.BehaviorWeight),
		EmotionalResponse:   scale(source.EmotionalResponse, rule.ValueWeight),
		RelationshipPattern: scale(source.RelationshipPattern, rule.RelationshipWeight),
	}
}

// FusionInput 是融合计算的一个来源。
type FusionInput struct {
	Character CharacterDNA
	Fusion    FusionSource
}

// FuseCharacters 把多个来源人物的 DNA 融合成一个新 DNA，并给出每个维度的来源说明。
//
// 规则：每个维度候选值 = 来源该维度权重 × 来源整体权重 ÷ 100，取最大者；
// 同分保留先出现的来源（结果可复现，不引入随机）。
func FuseCharacters(sources []FusionInput) (CharacterDNA, []FusionAttribution) {
	pick := map[string]func(CharacterDNA) DNADimension{
		"personality":          func(d CharacterDNA) DNADimension { return d.Personality },
		"values":               func(d CharacterDNA) DNADimension { return d.Values },
		"motivation":           func(d CharacterDNA) DNADimension { return d.Motivation },
		"behavior":             func(d CharacterDNA) DNADimension { return d.Behavior },
		"speech_style":         func(d CharacterDNA) DNADimension { return d.SpeechStyle },
		"background":           func(d CharacterDNA) DNADimension { return d.Background },
		"ability":              func(d CharacterDNA) DNADimension { return d.Ability },
		"decision_style":       func(d CharacterDNA) DNADimension { return d.DecisionStyle },
		"conflict_response":    func(d CharacterDNA) DNADimension { return d.ConflictResponse },
		"emotional_response":   func(d CharacterDNA) DNADimension { return d.EmotionalResponse },
		"relationship_pattern": func(d CharacterDNA) DNADimension { return d.RelationshipPattern },
	}

	result := CharacterDNA{}
	attributions := make([]FusionAttribution, 0, len(pick))
	set := func(name string, dim DNADimension) {
		switch name {
		case "personality":
			result.Personality = dim
		case "values":
			result.Values = dim
		case "motivation":
			result.Motivation = dim
		case "behavior":
			result.Behavior = dim
		case "speech_style":
			result.SpeechStyle = dim
		case "background":
			result.Background = dim
		case "ability":
			result.Ability = dim
		case "decision_style":
			result.DecisionStyle = dim
		case "conflict_response":
			result.ConflictResponse = dim
		case "emotional_response":
			result.EmotionalResponse = dim
		case "relationship_pattern":
			result.RelationshipPattern = dim
		}
	}

	for name, get := range pick {
		best := DNADimension{}
		attr := FusionAttribution{Dimension: name}
		for _, src := range sources {
			dim := get(src.Character)
			if dim.Weight <= 0 || src.Fusion.Weight <= 0 {
				continue
			}
			effective := dim.Weight * src.Fusion.Weight / 100
			if effective > best.Weight {
				best = DNADimension{Text: dim.Text, Weight: effective}
				attr = FusionAttribution{
					Dimension: name, FromName: src.Fusion.Name, FromID: src.Fusion.CharacterID, Weight: effective,
				}
			}
		}
		if best.Weight > 0 {
			set(name, best)
			attributions = append(attributions, attr)
		}
	}
	return result, attributions
}
