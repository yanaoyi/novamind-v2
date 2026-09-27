package service

import (
	"context"
	"errors"
	"strings"

	"github.com/yanaoyi/novamindv2/backend/internal/domain"
	"github.com/yanaoyi/novamindv2/backend/internal/repository"
)

// OriginalEventRepository 是事件/时间线/剧情弧 service 需要的仓储能力。
type OriginalEventRepository interface {
	CreateEvent(ctx context.Context, e *domain.OriginalEvent) error
	GetEvent(ctx context.Context, id string) (*domain.OriginalEvent, error)
	GetEventsByIDs(ctx context.Context, ids []string) ([]domain.OriginalEvent, error)
	ListEvents(ctx context.Context, workID string, f repository.EventFilter) ([]domain.OriginalEvent, int64, error)
	UpdateEvent(ctx context.Context, e *domain.OriginalEvent) error
	DeleteEvent(ctx context.Context, id string) error

	UpsertTimeline(ctx context.Context, t *domain.OriginalTimeline) error
	GetTimeline(ctx context.Context, workID string) (*domain.OriginalTimeline, error)
	ListTimelineEntries(ctx context.Context, timelineID string) ([]domain.TimelineEntry, error)
	ReplaceTimelineEntries(ctx context.Context, timelineID string, entries []domain.TimelineEntry) error

	CreatePlotArc(ctx context.Context, p *domain.PlotArc) error
	ListPlotArcs(ctx context.Context, workID string) ([]domain.PlotArc, error)
	GetPlotArc(ctx context.Context, id string) (*domain.PlotArc, error)
	UpdatePlotArc(ctx context.Context, p *domain.PlotArc) error
	DeletePlotArc(ctx context.Context, id string) error
}

// CharacterLookup 用于校验事件参与者属于同一部原著。
type CharacterLookup interface {
	GetCharacter(ctx context.Context, id string) (*domain.OriginalCharacter, error)
}

// LocationLookup 用于校验事件地点属于同一部原著的世界。
type LocationLookup interface {
	GetWorld(ctx context.Context, workID string) (*domain.OriginalWorld, error)
	GetLocation(ctx context.Context, id string) (*domain.Location, error)
}

// OriginalEventService 是事件 / 时间线 / 剧情弧的业务服务。
type OriginalEventService struct {
	repo       OriginalEventRepository
	works      WorkLookup
	characters CharacterLookup
	locations  LocationLookup
}

// NewOriginalEventService 构建服务。
func NewOriginalEventService(
	repo OriginalEventRepository,
	works WorkLookup,
	characters CharacterLookup,
	locations LocationLookup,
) *OriginalEventService {
	return &OriginalEventService{repo: repo, works: works, characters: characters, locations: locations}
}

// EventInput 是事件创建/更新入参。
type EventInput struct {
	Title        string
	Description  string
	ChapterNo    *int
	TimeOrder    int
	Participants []string
	LocationID   *string
	LocationText string
	Consequences string
	Importance   int
}

// ListEvents 列出某原著事件。
func (s *OriginalEventService) ListEvents(ctx context.Context, workID string, page, pageSize int, keyword string) ([]domain.OriginalEvent, int64, error) {
	if _, err := s.works.GetWorkByID(ctx, workID); err != nil {
		return nil, 0, err
	}
	items, total, err := s.repo.ListEvents(ctx, workID, repository.EventFilter{
		Keyword: strings.TrimSpace(keyword), Page: page, PageSize: pageSize,
	})
	if err != nil {
		return nil, 0, err
	}
	if items == nil {
		items = []domain.OriginalEvent{}
	}
	return items, total, nil
}

// CreateEvent 新增事件；校验参与者与地点都属于这部原著。
func (s *OriginalEventService) CreateEvent(ctx context.Context, workID string, in EventInput) (*domain.OriginalEvent, error) {
	if _, err := s.works.GetWorkByID(ctx, workID); err != nil {
		return nil, err
	}
	if err := s.assertRefs(ctx, workID, in.Participants, in.LocationID); err != nil {
		return nil, err
	}

	event := &domain.OriginalEvent{
		OriginalWorkID: workID,
		Title:          in.Title,
		Description:    in.Description,
		ChapterNo:      in.ChapterNo,
		TimeOrder:      in.TimeOrder,
		Participants:   in.Participants,
		LocationID:     in.LocationID,
		LocationText:   in.LocationText,
		Consequences:   in.Consequences,
		Importance:     in.Importance,
	}
	event.Normalize()
	if err := event.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.CreateEvent(ctx, event); err != nil {
		return nil, err
	}
	return event, nil
}

// GetEventByID 取事件详情。
func (s *OriginalEventService) GetEventByID(ctx context.Context, id string) (*domain.OriginalEvent, error) {
	if strings.TrimSpace(id) == "" {
		return nil, domain.ErrEventNotFound
	}
	return s.repo.GetEvent(ctx, id)
}

// UpdateEvent 更新事件。
func (s *OriginalEventService) UpdateEvent(ctx context.Context, id string, in EventInput) (*domain.OriginalEvent, error) {
	event, err := s.repo.GetEvent(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.assertRefs(ctx, event.OriginalWorkID, in.Participants, in.LocationID); err != nil {
		return nil, err
	}
	event.Title = in.Title
	event.Description = in.Description
	event.ChapterNo = in.ChapterNo
	event.TimeOrder = in.TimeOrder
	event.Participants = in.Participants
	event.LocationID = in.LocationID
	event.LocationText = in.LocationText
	event.Consequences = in.Consequences
	if in.Importance > 0 {
		event.Importance = in.Importance
	}
	event.Normalize()
	if err := event.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateEvent(ctx, event); err != nil {
		return nil, err
	}
	return event, nil
}

// DeleteEvent 删除事件（连带移除其时间线条目）。
func (s *OriginalEventService) DeleteEvent(ctx context.Context, id string) error {
	return s.repo.DeleteEvent(ctx, id)
}

// assertRefs 校验参与者与地点引用是否属于同一部原著。
func (s *OriginalEventService) assertRefs(ctx context.Context, workID string, participants []string, locationID *string) error {
	for _, id := range participants {
		if strings.TrimSpace(id) == "" {
			continue
		}
		character, err := s.characters.GetCharacter(ctx, id)
		if err != nil {
			if errors.Is(err, domain.ErrCharacterNotFound) {
				return domain.ErrEventParticipant
			}
			return err
		}
		if character.OriginalWorkID != workID {
			return domain.ErrEventParticipant
		}
	}
	if locationID != nil && strings.TrimSpace(*locationID) != "" {
		world, err := s.locations.GetWorld(ctx, workID)
		if err != nil {
			if errors.Is(err, domain.ErrWorldNotFound) {
				return domain.ErrLocationNotFound
			}
			return err
		}
		loc, err := s.locations.GetLocation(ctx, *locationID)
		if err != nil {
			return err
		}
		if loc.WorldID != world.ID {
			return domain.ErrLocationNotFound
		}
	}
	return nil
}

// TimelineEntryDetail 是时间线条目 + 事件本体。
type TimelineEntryDetail struct {
	Sequence  int
	TimeLabel string
	Duration  string
	Event     domain.OriginalEvent
}

// TimelineDetail 是时间线详情。
type TimelineDetail struct {
	Timeline domain.OriginalTimeline
	Entries  []TimelineEntryDetail
}

// GetTimeline 取某原著的时间线详情（时间线不存在时自动创建空时间线）。
func (s *OriginalEventService) GetTimeline(ctx context.Context, workID string) (*TimelineDetail, error) {
	if _, err := s.works.GetWorkByID(ctx, workID); err != nil {
		return nil, err
	}
	timeline, err := s.ensureTimeline(ctx, workID)
	if err != nil {
		return nil, err
	}
	entries, err := s.repo.ListTimelineEntries(ctx, timeline.ID)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(entries))
	for _, e := range entries {
		ids = append(ids, e.EventID)
	}
	events, err := s.repo.GetEventsByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]domain.OriginalEvent, len(events))
	for _, e := range events {
		byID[e.ID] = e
	}
	detail := &TimelineDetail{Timeline: *timeline, Entries: make([]TimelineEntryDetail, 0, len(entries))}
	for _, e := range entries {
		event, ok := byID[e.EventID]
		if !ok {
			continue // 事件被删则条目已被连带清理，这里兜底跳过
		}
		detail.Entries = append(detail.Entries, TimelineEntryDetail{
			Sequence: e.Sequence, TimeLabel: e.TimeLabel, Duration: e.Duration, Event: event,
		})
	}
	return detail, nil
}

// TimelineItemInput 是设置时间线顺序时的一项。
type TimelineItemInput struct {
	EventID   string
	TimeLabel string
	Duration  string
}

// SetTimelineOrder 用给定顺序整体替换时间线条目（校验所有事件属于该原著）。
func (s *OriginalEventService) SetTimelineOrder(ctx context.Context, workID string, items []TimelineItemInput) (*TimelineDetail, error) {
	if _, err := s.works.GetWorkByID(ctx, workID); err != nil {
		return nil, err
	}
	timeline, err := s.ensureTimeline(ctx, workID)
	if err != nil {
		return nil, err
	}
	if len(items) > 0 {
		ids := make([]string, 0, len(items))
		for _, it := range items {
			if strings.TrimSpace(it.EventID) == "" {
				return nil, domain.ErrTimelineOrderBad
			}
			ids = append(ids, it.EventID)
		}
		events, err := s.repo.GetEventsByIDs(ctx, ids)
		if err != nil {
			return nil, err
		}
		if len(events) != len(ids) {
			return nil, domain.ErrEventNotFound
		}
		for _, e := range events {
			if e.OriginalWorkID != workID {
				return nil, domain.ErrEventNotFound
			}
		}
	}

	entries := make([]domain.TimelineEntry, 0, len(items))
	for _, it := range items {
		entry := domain.TimelineEntry{EventID: it.EventID, TimeLabel: it.TimeLabel, Duration: it.Duration}
		entry.Normalize()
		entries = append(entries, entry)
	}
	if err := s.repo.ReplaceTimelineEntries(ctx, timeline.ID, entries); err != nil {
		return nil, err
	}
	return s.GetTimeline(ctx, workID)
}

// ensureTimeline 保证时间线存在。
func (s *OriginalEventService) ensureTimeline(ctx context.Context, workID string) (*domain.OriginalTimeline, error) {
	timeline, err := s.repo.GetTimeline(ctx, workID)
	if err == nil {
		return timeline, nil
	}
	if !errors.Is(err, domain.ErrTimelineNotFound) {
		return nil, err
	}
	created := &domain.OriginalTimeline{OriginalWorkID: workID}
	created.Normalize()
	if err := s.repo.UpsertTimeline(ctx, created); err != nil {
		return nil, err
	}
	return created, nil
}

// PlotArcInput 是剧情弧入参。
type PlotArcInput struct {
	Type         domain.PlotArcType
	Title        string
	Summary      string
	StartEventID *string
	EndEventID   *string
}

// CreatePlotArc 新增剧情弧；起止事件必须属于该原著。
func (s *OriginalEventService) CreatePlotArc(ctx context.Context, workID string, in PlotArcInput) (*domain.PlotArc, error) {
	if _, err := s.works.GetWorkByID(ctx, workID); err != nil {
		return nil, err
	}
	if err := s.assertArcEvents(ctx, workID, in.StartEventID, in.EndEventID); err != nil {
		return nil, err
	}
	arc := &domain.PlotArc{
		OriginalWorkID: workID, Type: in.Type, Title: in.Title, Summary: in.Summary,
		StartEventID: in.StartEventID, EndEventID: in.EndEventID,
	}
	arc.Normalize()
	if err := arc.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.CreatePlotArc(ctx, arc); err != nil {
		return nil, err
	}
	return arc, nil
}

// ListPlotArcs 列出剧情弧。
func (s *OriginalEventService) ListPlotArcs(ctx context.Context, workID string) ([]domain.PlotArc, error) {
	if _, err := s.works.GetWorkByID(ctx, workID); err != nil {
		return nil, err
	}
	items, err := s.repo.ListPlotArcs(ctx, workID)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []domain.PlotArc{}
	}
	return items, nil
}

// UpdatePlotArc 更新剧情弧。
func (s *OriginalEventService) UpdatePlotArc(ctx context.Context, id string, in PlotArcInput) (*domain.PlotArc, error) {
	arc, err := s.repo.GetPlotArc(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.assertArcEvents(ctx, arc.OriginalWorkID, in.StartEventID, in.EndEventID); err != nil {
		return nil, err
	}
	arc.Type = in.Type
	arc.Title = in.Title
	arc.Summary = in.Summary
	arc.StartEventID = in.StartEventID
	arc.EndEventID = in.EndEventID
	arc.Normalize()
	if err := arc.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.UpdatePlotArc(ctx, arc); err != nil {
		return nil, err
	}
	return arc, nil
}

// DeletePlotArc 删除剧情弧。
func (s *OriginalEventService) DeletePlotArc(ctx context.Context, id string) error {
	return s.repo.DeletePlotArc(ctx, id)
}

func (s *OriginalEventService) assertArcEvents(ctx context.Context, workID string, ids ...*string) error {
	for _, id := range ids {
		if id == nil || strings.TrimSpace(*id) == "" {
			continue
		}
		event, err := s.repo.GetEvent(ctx, *id)
		if err != nil {
			return err
		}
		if event.OriginalWorkID != workID {
			return domain.ErrEventNotFound
		}
	}
	return nil
}
