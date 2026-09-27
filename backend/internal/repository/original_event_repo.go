package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/yanaoyi/novamindv2/backend/internal/domain"
)

type eventModel struct {
	ID             string         `gorm:"column:id;type:uuid;primaryKey"`
	OriginalWorkID string         `gorm:"column:original_work_id;type:uuid;not null"`
	Title          string         `gorm:"column:title;size:200;not null"`
	Description    string         `gorm:"column:description;not null;default:''"`
	ChapterNo      *int           `gorm:"column:chapter_no"`
	TimeOrder      int            `gorm:"column:time_order;not null;default:0"`
	Participants   string         `gorm:"column:participants;type:jsonb;not null;default:'[]'"`
	LocationID     *string        `gorm:"column:location_id;type:uuid"`
	LocationText   string         `gorm:"column:location_text;size:200;not null;default:''"`
	Consequences   string         `gorm:"column:consequences;not null;default:''"`
	Importance     int            `gorm:"column:importance;not null;default:3"`
	Source         string         `gorm:"column:source;size:20;not null;default:MANUAL"`
	CreatedAt      time.Time      `gorm:"column:created_at;not null"`
	UpdatedAt      time.Time      `gorm:"column:updated_at;not null"`
	DeletedAt      gorm.DeletedAt `gorm:"column:deleted_at;index"`
}

func (eventModel) TableName() string { return "original_events" }

type timelineModel struct {
	ID             string         `gorm:"column:id;type:uuid;primaryKey"`
	OriginalWorkID string         `gorm:"column:original_work_id;type:uuid;not null"`
	Name           string         `gorm:"column:name;size:200;not null;default:'主线时间线'"`
	Description    string         `gorm:"column:description;not null;default:''"`
	CreatedAt      time.Time      `gorm:"column:created_at;not null"`
	UpdatedAt      time.Time      `gorm:"column:updated_at;not null"`
	DeletedAt      gorm.DeletedAt `gorm:"column:deleted_at;index"`
}

func (timelineModel) TableName() string { return "original_timelines" }

type timelineEntryModel struct {
	ID         string         `gorm:"column:id;type:uuid;primaryKey"`
	TimelineID string         `gorm:"column:timeline_id;type:uuid;not null"`
	EventID    string         `gorm:"column:event_id;type:uuid;not null"`
	Sequence   int            `gorm:"column:sequence;not null"`
	TimeLabel  string         `gorm:"column:time_label;size:120;not null;default:''"`
	Duration   string         `gorm:"column:duration;size:120;not null;default:''"`
	CreatedAt  time.Time      `gorm:"column:created_at;not null"`
	UpdatedAt  time.Time      `gorm:"column:updated_at;not null"`
	DeletedAt  gorm.DeletedAt `gorm:"column:deleted_at;index"`
}

func (timelineEntryModel) TableName() string { return "timeline_events" }

type plotArcModel struct {
	ID             string         `gorm:"column:id;type:uuid;primaryKey"`
	OriginalWorkID string         `gorm:"column:original_work_id;type:uuid;not null"`
	Type           string         `gorm:"column:type;size:30;not null;default:main"`
	Title          string         `gorm:"column:title;size:200;not null"`
	Summary        string         `gorm:"column:summary;not null;default:''"`
	StartEventID   *string        `gorm:"column:start_event_id;type:uuid"`
	EndEventID     *string        `gorm:"column:end_event_id;type:uuid"`
	CreatedAt      time.Time      `gorm:"column:created_at;not null"`
	UpdatedAt      time.Time      `gorm:"column:updated_at;not null"`
	DeletedAt      gorm.DeletedAt `gorm:"column:deleted_at;index"`
}

func (plotArcModel) TableName() string { return "plot_arcs" }

// EventFilter 是事件列表查询条件。
type EventFilter struct {
	Keyword  string
	Page     int
	PageSize int
}

// OriginalEventRepo 是事件 / 时间线 / 剧情弧仓储。
type OriginalEventRepo struct {
	db *gorm.DB
}

// NewOriginalEventRepo 构建仓储。
func NewOriginalEventRepo(db *gorm.DB) *OriginalEventRepo {
	return &OriginalEventRepo{db: db}
}

// ---------- 事件 ----------

// CreateEvent 新增事件。
func (r *OriginalEventRepo) CreateEvent(ctx context.Context, e *domain.OriginalEvent) error {
	if _, err := uuid.Parse(e.OriginalWorkID); err != nil {
		return domain.ErrOriginalNotFound
	}
	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("生成事件 ID 失败: %w", err)
	}
	e.ID = id.String()
	now := time.Now().UTC()
	e.CreatedAt, e.UpdatedAt = now, now

	m, err := toEventModel(e)
	if err != nil {
		return err
	}
	if err := r.db.WithContext(ctx).Create(&m).Error; err != nil {
		if isForeignKeyViolation(err) {
			return domain.ErrOriginalNotFound
		}
		return fmt.Errorf("创建事件失败: %w", err)
	}
	return nil
}

// GetEvent 取事件。
func (r *OriginalEventRepo) GetEvent(ctx context.Context, id string) (*domain.OriginalEvent, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, domain.ErrEventNotFound
	}
	var m eventModel
	if err := r.db.WithContext(ctx).First(&m, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrEventNotFound
		}
		return nil, fmt.Errorf("查询事件失败: %w", err)
	}
	e, err := toDomainEvent(m)
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// GetEventsByIDs 按 ID 批量取事件（返回顺序与传入一致，缺失的跳过）。
func (r *OriginalEventRepo) GetEventsByIDs(ctx context.Context, ids []string) ([]domain.OriginalEvent, error) {
	if len(ids) == 0 {
		return []domain.OriginalEvent{}, nil
	}
	var models []eventModel
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&models).Error; err != nil {
		return nil, fmt.Errorf("批量查询事件失败: %w", err)
	}
	byID := make(map[string]domain.OriginalEvent, len(models))
	for _, m := range models {
		e, err := toDomainEvent(m)
		if err != nil {
			return nil, err
		}
		byID[e.ID] = e
	}
	out := make([]domain.OriginalEvent, 0, len(ids))
	for _, id := range ids {
		if e, ok := byID[id]; ok {
			out = append(out, e)
		}
	}
	return out, nil
}

// ListEvents 分页列出某原著的事件（按时间顺序号）。
func (r *OriginalEventRepo) ListEvents(ctx context.Context, workID string, f EventFilter) ([]domain.OriginalEvent, int64, error) {
	if _, err := uuid.Parse(workID); err != nil {
		return nil, 0, domain.ErrOriginalNotFound
	}
	query := r.db.WithContext(ctx).Model(&eventModel{}).Where("original_work_id = ?", workID)
	if f.Keyword != "" {
		query = query.Where("title ILIKE ? OR description ILIKE ?", "%"+f.Keyword+"%", "%"+f.Keyword+"%")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("统计事件失败: %w", err)
	}

	page, pageSize := f.Page, f.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 100
	}
	if pageSize > 500 {
		pageSize = 500
	}

	var models []eventModel
	if err := query.Order("time_order ASC, chapter_no ASC NULLS LAST, created_at ASC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&models).Error; err != nil {
		return nil, 0, fmt.Errorf("查询事件失败: %w", err)
	}

	out := make([]domain.OriginalEvent, 0, len(models))
	for _, m := range models {
		e, err := toDomainEvent(m)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, e)
	}
	return out, total, nil
}

// UpdateEvent 更新事件。
func (r *OriginalEventRepo) UpdateEvent(ctx context.Context, e *domain.OriginalEvent) error {
	m, err := toEventModel(e)
	if err != nil {
		return err
	}
	m.UpdatedAt = time.Now().UTC()

	res := r.db.WithContext(ctx).Model(&eventModel{}).Where("id = ?", e.ID).Updates(map[string]any{
		"title":         m.Title,
		"description":   m.Description,
		"chapter_no":    m.ChapterNo,
		"time_order":    m.TimeOrder,
		"participants":  m.Participants,
		"location_id":   m.LocationID,
		"location_text": m.LocationText,
		"consequences":  m.Consequences,
		"importance":    m.Importance,
		"updated_at":    m.UpdatedAt,
	})
	if res.Error != nil {
		if isForeignKeyViolation(res.Error) {
			return domain.ErrLocationNotFound
		}
		return fmt.Errorf("更新事件失败: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		if _, err := r.GetEvent(ctx, e.ID); err != nil {
			return err
		}
	}
	e.UpdatedAt = m.UpdatedAt
	return nil
}

// DeleteEvent 软删除事件，并连带移除它在时间线上的条目。
func (r *OriginalEventRepo) DeleteEvent(ctx context.Context, id string) error {
	if _, err := uuid.Parse(id); err != nil {
		return domain.ErrEventNotFound
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("event_id = ?", id).Delete(&timelineEntryModel{}).Error; err != nil {
			return fmt.Errorf("清理时间线条目失败: %w", err)
		}
		res := tx.Where("id = ?", id).Delete(&eventModel{})
		if res.Error != nil {
			return fmt.Errorf("删除事件失败: %w", res.Error)
		}
		if res.RowsAffected == 0 {
			return domain.ErrEventNotFound
		}
		return nil
	})
}

// ---------- 时间线 ----------

// UpsertTimeline 创建或更新某原著的时间线（一部原著一条）。
func (r *OriginalEventRepo) UpsertTimeline(ctx context.Context, t *domain.OriginalTimeline) error {
	if _, err := uuid.Parse(t.OriginalWorkID); err != nil {
		return domain.ErrOriginalNotFound
	}
	var existing timelineModel
	err := r.db.WithContext(ctx).First(&existing, "original_work_id = ?", t.OriginalWorkID).Error
	now := time.Now().UTC()

	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		id, genErr := uuid.NewV7()
		if genErr != nil {
			return fmt.Errorf("生成时间线 ID 失败: %w", genErr)
		}
		t.ID = id.String()
		t.CreatedAt, t.UpdatedAt = now, now
		m := timelineModel{
			ID: t.ID, OriginalWorkID: t.OriginalWorkID, Name: t.Name,
			Description: t.Description, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt,
		}
		if err := r.db.WithContext(ctx).Create(&m).Error; err != nil {
			if isForeignKeyViolation(err) {
				return domain.ErrOriginalNotFound
			}
			return fmt.Errorf("创建时间线失败: %w", err)
		}
		return nil
	case err != nil:
		return fmt.Errorf("查询时间线失败: %w", err)
	}

	res := r.db.WithContext(ctx).Model(&timelineModel{}).Where("id = ?", existing.ID).Updates(map[string]any{
		"name":        t.Name,
		"description": t.Description,
		"updated_at":  now,
	})
	if res.Error != nil {
		return fmt.Errorf("更新时间线失败: %w", res.Error)
	}
	t.ID = existing.ID
	t.CreatedAt = existing.CreatedAt
	t.UpdatedAt = now
	return nil
}

// GetTimeline 取某原著的时间线。
func (r *OriginalEventRepo) GetTimeline(ctx context.Context, workID string) (*domain.OriginalTimeline, error) {
	if _, err := uuid.Parse(workID); err != nil {
		return nil, domain.ErrTimelineNotFound
	}
	var m timelineModel
	if err := r.db.WithContext(ctx).First(&m, "original_work_id = ?", workID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrTimelineNotFound
		}
		return nil, fmt.Errorf("查询时间线失败: %w", err)
	}
	t := domain.OriginalTimeline{
		ID: m.ID, OriginalWorkID: m.OriginalWorkID, Name: m.Name, Description: m.Description,
		CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}
	if m.DeletedAt.Valid {
		ts := m.DeletedAt.Time
		t.DeletedAt = &ts
	}
	return &t, nil
}

// ListTimelineEntries 列出时间线条目（按顺序）。
func (r *OriginalEventRepo) ListTimelineEntries(ctx context.Context, timelineID string) ([]domain.TimelineEntry, error) {
	var models []timelineEntryModel
	if err := r.db.WithContext(ctx).Where("timeline_id = ?", timelineID).
		Order("sequence ASC").Find(&models).Error; err != nil {
		return nil, fmt.Errorf("查询时间线条目失败: %w", err)
	}
	out := make([]domain.TimelineEntry, 0, len(models))
	for _, m := range models {
		out = append(out, toDomainTimelineEntry(m))
	}
	return out, nil
}

// ReplaceTimelineEntries 用给定顺序整体替换时间线条目（单事务，幂等）。
// 传入的 entries 会按切片顺序重新编号为 1..N。
func (r *OriginalEventRepo) ReplaceTimelineEntries(ctx context.Context, timelineID string, entries []domain.TimelineEntry) error {
	if _, err := uuid.Parse(timelineID); err != nil {
		return domain.ErrTimelineNotFound
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var exists int64
		if err := tx.Model(&timelineModel{}).Where("id = ?", timelineID).Count(&exists).Error; err != nil {
			return fmt.Errorf("校验时间线失败: %w", err)
		}
		if exists == 0 {
			return domain.ErrTimelineNotFound
		}
		if err := tx.Where("timeline_id = ?", timelineID).Delete(&timelineEntryModel{}).Error; err != nil {
			return fmt.Errorf("清理旧时间线条目失败: %w", err)
		}

		now := time.Now().UTC()
		models := make([]timelineEntryModel, 0, len(entries))
		seen := make(map[string]bool, len(entries))
		for i, e := range entries {
			if seen[e.EventID] {
				return domain.ErrTimelineEventDup
			}
			seen[e.EventID] = true
			id, err := uuid.NewV7()
			if err != nil {
				return fmt.Errorf("生成时间线条目 ID 失败: %w", err)
			}
			models = append(models, timelineEntryModel{
				ID: id.String(), TimelineID: timelineID, EventID: e.EventID,
				Sequence: i + 1, TimeLabel: e.TimeLabel, Duration: e.Duration,
				CreatedAt: now, UpdatedAt: now,
			})
		}
		if len(models) > 0 {
			if err := tx.CreateInBatches(&models, 200).Error; err != nil {
				if isUniqueViolation(err) {
					return domain.ErrTimelineEventDup
				}
				if isForeignKeyViolation(err) {
					return domain.ErrEventNotFound
				}
				return fmt.Errorf("写入时间线条目失败: %w", err)
			}
		}
		return nil
	})
}

// ---------- 剧情弧 ----------

// CreatePlotArc 新增剧情弧。
func (r *OriginalEventRepo) CreatePlotArc(ctx context.Context, p *domain.PlotArc) error {
	if _, err := uuid.Parse(p.OriginalWorkID); err != nil {
		return domain.ErrOriginalNotFound
	}
	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("生成剧情弧 ID 失败: %w", err)
	}
	p.ID = id.String()
	now := time.Now().UTC()
	p.CreatedAt, p.UpdatedAt = now, now

	m := plotArcModel{
		ID: p.ID, OriginalWorkID: p.OriginalWorkID, Type: string(p.Type), Title: p.Title,
		Summary: p.Summary, StartEventID: p.StartEventID, EndEventID: p.EndEventID,
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
	if err := r.db.WithContext(ctx).Create(&m).Error; err != nil {
		if isForeignKeyViolation(err) {
			return domain.ErrEventNotFound
		}
		return fmt.Errorf("创建剧情弧失败: %w", err)
	}
	return nil
}

// ListPlotArcs 列出某原著的剧情弧。
func (r *OriginalEventRepo) ListPlotArcs(ctx context.Context, workID string) ([]domain.PlotArc, error) {
	if _, err := uuid.Parse(workID); err != nil {
		return nil, domain.ErrOriginalNotFound
	}
	var models []plotArcModel
	if err := r.db.WithContext(ctx).Where("original_work_id = ?", workID).
		Order("created_at ASC").Find(&models).Error; err != nil {
		return nil, fmt.Errorf("查询剧情弧失败: %w", err)
	}
	out := make([]domain.PlotArc, 0, len(models))
	for _, m := range models {
		out = append(out, toDomainPlotArc(m))
	}
	return out, nil
}

// GetPlotArc 取剧情弧。
func (r *OriginalEventRepo) GetPlotArc(ctx context.Context, id string) (*domain.PlotArc, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, domain.ErrPlotArcNotFound
	}
	var m plotArcModel
	if err := r.db.WithContext(ctx).First(&m, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrPlotArcNotFound
		}
		return nil, fmt.Errorf("查询剧情弧失败: %w", err)
	}
	p := toDomainPlotArc(m)
	return &p, nil
}

// UpdatePlotArc 更新剧情弧。
func (r *OriginalEventRepo) UpdatePlotArc(ctx context.Context, p *domain.PlotArc) error {
	now := time.Now().UTC()
	res := r.db.WithContext(ctx).Model(&plotArcModel{}).Where("id = ?", p.ID).Updates(map[string]any{
		"type":           string(p.Type),
		"title":          p.Title,
		"summary":        p.Summary,
		"start_event_id": p.StartEventID,
		"end_event_id":   p.EndEventID,
		"updated_at":     now,
	})
	if res.Error != nil {
		if isForeignKeyViolation(res.Error) {
			return domain.ErrEventNotFound
		}
		return fmt.Errorf("更新剧情弧失败: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		if _, err := r.GetPlotArc(ctx, p.ID); err != nil {
			return err
		}
	}
	p.UpdatedAt = now
	return nil
}

// DeletePlotArc 软删除剧情弧。
func (r *OriginalEventRepo) DeletePlotArc(ctx context.Context, id string) error {
	if _, err := uuid.Parse(id); err != nil {
		return domain.ErrPlotArcNotFound
	}
	res := r.db.WithContext(ctx).Where("id = ?", id).Delete(&plotArcModel{})
	if res.Error != nil {
		return fmt.Errorf("删除剧情弧失败: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.ErrPlotArcNotFound
	}
	return nil
}

// ---------- 转换 ----------

func toEventModel(e *domain.OriginalEvent) (eventModel, error) {
	participants, err := json.Marshal(e.Participants)
	if err != nil {
		return eventModel{}, fmt.Errorf("序列化事件参与者失败: %w", err)
	}
	if e.Participants == nil {
		participants = []byte("[]")
	}
	return eventModel{
		ID: e.ID, OriginalWorkID: e.OriginalWorkID, Title: e.Title, Description: e.Description,
		ChapterNo: e.ChapterNo, TimeOrder: e.TimeOrder, Participants: string(participants),
		LocationID: e.LocationID, LocationText: e.LocationText, Consequences: e.Consequences,
		Importance: e.Importance, Source: string(e.Source),
		CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt,
	}, nil
}

func toDomainEvent(m eventModel) (domain.OriginalEvent, error) {
	e := domain.OriginalEvent{
		ID: m.ID, OriginalWorkID: m.OriginalWorkID, Title: m.Title, Description: m.Description,
		ChapterNo: m.ChapterNo, TimeOrder: m.TimeOrder, LocationID: m.LocationID,
		LocationText: m.LocationText, Consequences: m.Consequences,
		Importance: m.Importance, Source: domain.CharacterSource(m.Source),
		CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}
	if m.Participants != "" {
		if err := json.Unmarshal([]byte(m.Participants), &e.Participants); err != nil {
			return domain.OriginalEvent{}, fmt.Errorf("解析事件参与者失败: %w", err)
		}
	}
	if e.Participants == nil {
		e.Participants = []string{}
	}
	if m.DeletedAt.Valid {
		t := m.DeletedAt.Time
		e.DeletedAt = &t
	}
	return e, nil
}

func toDomainTimelineEntry(m timelineEntryModel) domain.TimelineEntry {
	entry := domain.TimelineEntry{
		ID: m.ID, TimelineID: m.TimelineID, EventID: m.EventID, Sequence: m.Sequence,
		TimeLabel: m.TimeLabel, Duration: m.Duration,
		CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}
	if m.DeletedAt.Valid {
		t := m.DeletedAt.Time
		entry.DeletedAt = &t
	}
	return entry
}

func toDomainPlotArc(m plotArcModel) domain.PlotArc {
	p := domain.PlotArc{
		ID: m.ID, OriginalWorkID: m.OriginalWorkID, Type: domain.PlotArcType(m.Type),
		Title: m.Title, Summary: m.Summary, StartEventID: m.StartEventID, EndEventID: m.EndEventID,
		CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}
	if m.DeletedAt.Valid {
		t := m.DeletedAt.Time
		p.DeletedAt = &t
	}
	return p
}
