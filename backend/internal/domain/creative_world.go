package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// 二创世界 / 分叉点 / 二创时间线相关错误。
var (
	ErrCreativeWorldNotFound     = errors.New("二创世界不存在")
	ErrCreativeWorldModeBad      = errors.New("世界继承模式非法")
	ErrCreativeWorldRuleNotFound = errors.New("二创世界规则不存在")
	ErrCreativeWorldRuleName     = errors.New("规则名称不能为空")
	ErrCreativeWorldRuleStatus   = errors.New("规则状态非法")
	ErrCreativeWorldRuleDup      = errors.New("同名规则已存在")
	ErrDivergenceNotFound        = errors.New("分叉点不存在")
	ErrDivergenceSourceInvalid   = errors.New("分叉点必须指向该原著的事件或章节")
	ErrCreativeTimelineBad       = errors.New("二创时间线数据不合法")
)

// WorldInheritanceMode 是二创世界的继承模式（规格书 §21）。
type WorldInheritanceMode string

const (
	WorldInheritFull     WorldInheritanceMode = "FULL"
	WorldInheritPartial  WorldInheritanceMode = "PARTIAL"
	WorldInheritModified WorldInheritanceMode = "MODIFIED"
	WorldInheritNew      WorldInheritanceMode = "NEW"
)

// Valid 判断继承模式是否合法。
func (m WorldInheritanceMode) Valid() bool {
	switch m {
	case WorldInheritFull, WorldInheritPartial, WorldInheritModified, WorldInheritNew:
		return true
	default:
		return false
	}
}

// CopiesRules 判断该模式是否默认把原著规则整套带过来。
func (m WorldInheritanceMode) CopiesRules() bool {
	return m == WorldInheritFull || m == WorldInheritPartial
}

// CreativeWorld 是二创世界（规格书 §21）。
type CreativeWorld struct {
	ID              string
	CreativeWorkID  string
	SourceWorldID   *string
	InheritanceMode WorldInheritanceMode
	Name            string
	Description     string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	DeletedAt       *time.Time
}

// Normalize 清洗输入并补默认值。
func (w *CreativeWorld) Normalize() {
	w.Name = strings.TrimSpace(w.Name)
	w.Description = strings.TrimSpace(w.Description)
	if w.InheritanceMode == "" {
		w.InheritanceMode = WorldInheritFull
	}
}

// Validate 校验二创世界。
func (w *CreativeWorld) Validate() error {
	if !w.InheritanceMode.Valid() {
		return fmt.Errorf("%w: %s", ErrCreativeWorldModeBad, w.InheritanceMode)
	}
	return nil
}

// CreativeWorldRuleStatus 是二创世界规则的状态（规格书 §22）。
type CreativeWorldRuleStatus string

const (
	RuleInherited CreativeWorldRuleStatus = "INHERITED"
	RuleModified  CreativeWorldRuleStatus = "MODIFIED"
	RuleRemoved   CreativeWorldRuleStatus = "REMOVED"
	RuleNew       CreativeWorldRuleStatus = "NEW"
)

// Valid 判断规则状态是否合法。
func (s CreativeWorldRuleStatus) Valid() bool {
	switch s {
	case RuleInherited, RuleModified, RuleRemoved, RuleNew:
		return true
	default:
		return false
	}
}

// CreativeWorldRule 是二创世界规则。
type CreativeWorldRule struct {
	ID              string
	CreativeWorldID string
	SourceRuleID    *string
	Status          CreativeWorldRuleStatus
	Category        string
	Name            string
	Description     string
	Importance      int
	CreatedAt       time.Time
	UpdatedAt       time.Time
	DeletedAt       *time.Time
}

// Normalize 清洗输入并补默认值。
func (r *CreativeWorldRule) Normalize() {
	r.Category = strings.TrimSpace(r.Category)
	r.Name = strings.TrimSpace(r.Name)
	r.Description = strings.TrimSpace(r.Description)
	if r.Importance == 0 {
		r.Importance = 3
	}
	if r.Status == "" {
		r.Status = RuleNew
	}
}

// Validate 校验二创世界规则。
func (r *CreativeWorldRule) Validate() error {
	if strings.TrimSpace(r.Name) == "" {
		return ErrCreativeWorldRuleName
	}
	if !r.Status.Valid() {
		return fmt.Errorf("%w: %s", ErrCreativeWorldRuleStatus, r.Status)
	}
	if r.Importance < 1 || r.Importance > 5 {
		return ErrWorldRuleImportance
	}
	return nil
}

// DivergencePoint 是分叉点（规格书 §24）。
type DivergencePoint struct {
	ID                string
	CreativeWorkID    string
	OriginalChapterID *string
	OriginalEventID   *string
	TimeLabel         string
	Description       string
	CreatedAt         time.Time
	UpdatedAt         time.Time
	DeletedAt         *time.Time
}

// Normalize 清洗输入。
func (d *DivergencePoint) Normalize() {
	d.TimeLabel = strings.TrimSpace(d.TimeLabel)
	d.Description = strings.TrimSpace(d.Description)
}

// Validate 校验分叉点（至少要指出从哪里分叉）。
func (d *DivergencePoint) Validate() error {
	if (d.OriginalChapterID == nil || strings.TrimSpace(*d.OriginalChapterID) == "") &&
		(d.OriginalEventID == nil || strings.TrimSpace(*d.OriginalEventID) == "") {
		return ErrDivergenceSourceInvalid
	}
	return nil
}

// CreativeTimelineEventStatus 是二创时间线事件状态（规格书 §25）。
type CreativeTimelineEventStatus string

const (
	TimelineInherited CreativeTimelineEventStatus = "INHERITED"
	TimelineModified  CreativeTimelineEventStatus = "MODIFIED"
	TimelineNew       CreativeTimelineEventStatus = "NEW"
	TimelineRemoved   CreativeTimelineEventStatus = "REMOVED"
)

// Valid 判断时间线事件状态是否合法。
func (s CreativeTimelineEventStatus) Valid() bool {
	switch s {
	case TimelineInherited, TimelineModified, TimelineNew, TimelineRemoved:
		return true
	default:
		return false
	}
}

// CreativeTimelineEvent 是二创时间线上的一个事件。
type CreativeTimelineEvent struct {
	ID                    string
	CreativeWorkID        string
	SourceOriginalEventID *string
	Status                CreativeTimelineEventStatus
	Sequence              int
	TimeLabel             string
	Title                 string
	Description           string
	CreatedAt             time.Time
	UpdatedAt             time.Time
	DeletedAt             *time.Time
}

// Normalize 清洗输入并补默认值。
func (e *CreativeTimelineEvent) Normalize() {
	e.TimeLabel = strings.TrimSpace(e.TimeLabel)
	e.Title = strings.TrimSpace(e.Title)
	e.Description = strings.TrimSpace(e.Description)
	if e.Status == "" {
		if e.SourceOriginalEventID != nil {
			e.Status = TimelineInherited
		} else {
			e.Status = TimelineNew
		}
	}
}

// Validate 校验二创时间线事件。
func (e *CreativeTimelineEvent) Validate() error {
	if strings.TrimSpace(e.Title) == "" {
		return fmt.Errorf("%w：标题不能为空", ErrCreativeTimelineBad)
	}
	if !e.Status.Valid() {
		return fmt.Errorf("%w：状态非法 %s", ErrCreativeTimelineBad, e.Status)
	}
	return nil
}
