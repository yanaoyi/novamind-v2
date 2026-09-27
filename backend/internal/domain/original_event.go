package domain

import (
	"errors"
	"strings"
	"time"
)

// 事件 / 时间线 / 剧情弧相关错误。
var (
	ErrEventTitleEmpty    = errors.New("事件标题不能为空")
	ErrEventImportance    = errors.New("事件重要度必须是 1-5")
	ErrEventChapterNo     = errors.New("事件章节号必须为正整数")
	ErrEventNotFound      = errors.New("事件不存在")
	ErrEventParticipant   = errors.New("事件参与者必须是同一部原著中的人物")
	ErrTimelineNotFound   = errors.New("时间线不存在")
	ErrTimelineEventDup   = errors.New("该事件已在这条时间线上")
	ErrTimelineEventMiss  = errors.New("该事件不在这条时间线上")
	ErrTimelineOrderBad   = errors.New("时间线排序数据不合法")
	ErrPlotArcNotFound    = errors.New("剧情弧不存在")
	ErrPlotArcTitleEmpty  = errors.New("剧情弧标题不能为空")
	ErrPlotArcTypeInvalid = errors.New("剧情弧类型非法")
)

// PlotArcType 是剧情弧类型（规格书 §16）。
type PlotArcType string

const (
	PlotArcMain         PlotArcType = "main"
	PlotArcSubplot      PlotArcType = "subplot"
	PlotArcCharacter    PlotArcType = "character_arc"
	PlotArcRelationship PlotArcType = "relationship_arc"
	PlotArcWorld        PlotArcType = "world_arc"
)

// Valid 判断剧情弧类型是否合法。
func (t PlotArcType) Valid() bool {
	switch t {
	case PlotArcMain, PlotArcSubplot, PlotArcCharacter, PlotArcRelationship, PlotArcWorld:
		return true
	default:
		return false
	}
}

// OriginalEvent 是原著事件（规格书 §14）。
type OriginalEvent struct {
	ID             string
	OriginalWorkID string
	Title          string
	Description    string
	ChapterNo      *int
	TimeOrder      int
	Participants   []string // 人物 ID 列表
	LocationID     *string
	LocationText   string
	Consequences   string
	Importance     int
	Source         CharacterSource
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      *time.Time
}

// Normalize 清洗输入并补默认值。
func (e *OriginalEvent) Normalize() {
	e.Title = strings.TrimSpace(e.Title)
	e.Description = strings.TrimSpace(e.Description)
	e.LocationText = strings.TrimSpace(e.LocationText)
	e.Consequences = strings.TrimSpace(e.Consequences)
	if e.Importance == 0 {
		e.Importance = 3
	}
	if e.Source == "" {
		e.Source = SourceManual
	}
	if e.LocationID != nil && strings.TrimSpace(*e.LocationID) == "" {
		e.LocationID = nil
	}
	if e.Participants == nil {
		e.Participants = []string{}
	}
}

// Validate 校验事件（参与者是否属于同一部原著由 service 负责）。
func (e *OriginalEvent) Validate() error {
	if strings.TrimSpace(e.Title) == "" {
		return ErrEventTitleEmpty
	}
	if e.Importance < 1 || e.Importance > 5 {
		return ErrEventImportance
	}
	if e.ChapterNo != nil && *e.ChapterNo <= 0 {
		return ErrEventChapterNo
	}
	return nil
}

// OriginalTimeline 是原著时间线（V1 一部原著一条）。
type OriginalTimeline struct {
	ID             string
	OriginalWorkID string
	Name           string
	Description    string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      *time.Time
}

// Normalize 清洗输入。
func (t *OriginalTimeline) Normalize() {
	t.Name = strings.TrimSpace(t.Name)
	t.Description = strings.TrimSpace(t.Description)
	if t.Name == "" {
		t.Name = "主线时间线"
	}
}

// TimelineEntry 是时间线上的一个条目（事件 + 顺序 + 时间标签）。
type TimelineEntry struct {
	ID         string
	TimelineID string
	EventID    string
	Sequence   int
	TimeLabel  string
	Duration   string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	DeletedAt  *time.Time
}

// Normalize 清洗输入。
func (t *TimelineEntry) Normalize() {
	t.TimeLabel = strings.TrimSpace(t.TimeLabel)
	t.Duration = strings.TrimSpace(t.Duration)
}

// PlotArc 是剧情弧（规格书 §16）。
type PlotArc struct {
	ID             string
	OriginalWorkID string
	Type           PlotArcType
	Title          string
	Summary        string
	StartEventID   *string
	EndEventID     *string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      *time.Time
}

// Normalize 清洗输入并补默认值。
func (p *PlotArc) Normalize() {
	p.Title = strings.TrimSpace(p.Title)
	p.Summary = strings.TrimSpace(p.Summary)
	if p.Type == "" {
		p.Type = PlotArcMain
	}
	if p.StartEventID != nil && strings.TrimSpace(*p.StartEventID) == "" {
		p.StartEventID = nil
	}
	if p.EndEventID != nil && strings.TrimSpace(*p.EndEventID) == "" {
		p.EndEventID = nil
	}
}

// Validate 校验剧情弧。
func (p *PlotArc) Validate() error {
	if strings.TrimSpace(p.Title) == "" {
		return ErrPlotArcTitleEmpty
	}
	if !p.Type.Valid() {
		return ErrPlotArcTypeInvalid
	}
	return nil
}
