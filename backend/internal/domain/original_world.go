package domain

import (
	"errors"
	"strings"
	"time"
)

// 世界观相关错误。
var (
	ErrWorldNotFound       = errors.New("世界观不存在")
	ErrWorldRuleNotFound   = errors.New("世界规则不存在")
	ErrWorldRuleNameEmpty  = errors.New("规则名称不能为空")
	ErrWorldRuleDuplicate  = errors.New("同名规则已存在")
	ErrWorldRuleImportance = errors.New("规则重要度必须是 1-5")
	ErrLocationNotFound    = errors.New("地点不存在")
	ErrLocationNameEmpty   = errors.New("地点名称不能为空")
	ErrLocationDuplicate   = errors.New("同名地点已存在")
	ErrLocationParentCross = errors.New("上级地点必须属于同一个世界")
	ErrLocationSelfParent  = errors.New("地点不能以自己为上级")
	ErrLocationCycle       = errors.New("地点层级不能形成环")
	ErrFactionNotFound     = errors.New("势力不存在")
	ErrFactionNameEmpty    = errors.New("势力名称不能为空")
	ErrFactionDuplicate    = errors.New("同名势力已存在")
)

// OriginalWorld 是原著世界观（规格书 §13）。
type OriginalWorld struct {
	ID             string
	OriginalWorkID string
	Name           string
	Description    string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      *time.Time
}

// Normalize 清洗输入。
func (w *OriginalWorld) Normalize() {
	w.Name = strings.TrimSpace(w.Name)
	w.Description = strings.TrimSpace(w.Description)
}

// WorldRule 是世界规则。
type WorldRule struct {
	ID          string
	WorldID     string
	Category    string
	Name        string
	Description string
	Importance  int
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
}

// Normalize 清洗输入并补默认值。
func (r *WorldRule) Normalize() {
	r.Category = strings.TrimSpace(r.Category)
	r.Name = strings.TrimSpace(r.Name)
	r.Description = strings.TrimSpace(r.Description)
	if r.Importance == 0 {
		r.Importance = 3
	}
}

// Validate 校验规则。
func (r *WorldRule) Validate() error {
	if strings.TrimSpace(r.Name) == "" {
		return ErrWorldRuleNameEmpty
	}
	if r.Importance < 1 || r.Importance > 5 {
		return ErrWorldRuleImportance
	}
	return nil
}

// Location 是地点，支持父子层级（规格书 §13 Location）。
type Location struct {
	ID               string
	WorldID          string
	Name             string
	Type             string
	Description      string
	ParentLocationID *string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	DeletedAt        *time.Time
}

// Normalize 清洗输入。
func (l *Location) Normalize() {
	l.Name = strings.TrimSpace(l.Name)
	l.Type = strings.TrimSpace(l.Type)
	l.Description = strings.TrimSpace(l.Description)
	if l.ParentLocationID != nil && strings.TrimSpace(*l.ParentLocationID) == "" {
		l.ParentLocationID = nil
	}
}

// Validate 校验地点（不含层级环检测，环检测需要查库，放在 service）。
func (l *Location) Validate() error {
	if strings.TrimSpace(l.Name) == "" {
		return ErrLocationNameEmpty
	}
	if l.ParentLocationID != nil && *l.ParentLocationID == l.ID {
		return ErrLocationSelfParent
	}
	return nil
}

// Faction 是势力。
type Faction struct {
	ID            string
	WorldID       string
	Name          string
	Type          string
	Description   string
	Goals         string
	Relationships string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	DeletedAt     *time.Time
}

// Normalize 清洗输入。
func (f *Faction) Normalize() {
	f.Name = strings.TrimSpace(f.Name)
	f.Type = strings.TrimSpace(f.Type)
	f.Description = strings.TrimSpace(f.Description)
	f.Goals = strings.TrimSpace(f.Goals)
	f.Relationships = strings.TrimSpace(f.Relationships)
}

// Validate 校验势力。
func (f *Faction) Validate() error {
	if strings.TrimSpace(f.Name) == "" {
		return ErrFactionNameEmpty
	}
	return nil
}

// WorldSummary 是世界观概览（用于界面一眼看清规模）。
type WorldSummary struct {
	World     OriginalWorld
	Rules     int
	Locations int
	Factions  int
}
