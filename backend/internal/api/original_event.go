package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/yanaoyi/novamindv2/backend/internal/domain"
	"github.com/yanaoyi/novamindv2/backend/internal/service"
)

// ---------- 请求 DTO ----------

type eventRequest struct {
	Title        string   `json:"title" binding:"required" example:"母亲的意外"`
	Description  string   `json:"description"`
	ChapterNo    *int     `json:"chapter_no" example:"3"`
	TimeOrder    int      `json:"time_order" example:"10"`
	Participants []string `json:"participants" example:"[\"人物ID\"]"`
	LocationID   *string  `json:"location_id"`
	LocationText string   `json:"location_text" example:"老糖厂"`
	Consequences string   `json:"consequences" example:"林默决定回江城"`
	Importance   int      `json:"importance" example:"5"`
}

type timelineItemRequest struct {
	EventID   string `json:"event_id" binding:"required"`
	TimeLabel string `json:"time_label" example:"三年前·梅雨季"`
	Duration  string `json:"duration" example:"两天"`
}

type timelineOrderRequest struct {
	Items []timelineItemRequest `json:"items" binding:"required"`
}

type plotArcRequest struct {
	Type         string  `json:"type" binding:"omitempty,oneof=main subplot character_arc relationship_arc world_arc" example:"main"`
	Title        string  `json:"title" binding:"required" example:"老城改造之争"`
	Summary      string  `json:"summary"`
	StartEventID *string `json:"start_event_id"`
	EndEventID   *string `json:"end_event_id"`
}

// ---------- 响应 DTO ----------

// EventResponse 是事件对外表示。
type EventResponse struct {
	ID             string    `json:"id"`
	OriginalWorkID string    `json:"original_work_id"`
	Title          string    `json:"title"`
	Description    string    `json:"description"`
	ChapterNo      *int      `json:"chapter_no"`
	TimeOrder      int       `json:"time_order"`
	Participants   []string  `json:"participants"`
	LocationID     *string   `json:"location_id"`
	LocationText   string    `json:"location_text"`
	Consequences   string    `json:"consequences"`
	Importance     int       `json:"importance"`
	Source         string    `json:"source"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// EventListResponse 是事件列表响应。
type EventListResponse struct {
	Items    []EventResponse `json:"items"`
	Total    int64           `json:"total"`
	Page     int             `json:"page"`
	PageSize int             `json:"page_size"`
}

// TimelineEntryResponse 是时间线条目（含事件本体）。
type TimelineEntryResponse struct {
	Sequence  int           `json:"sequence"`
	TimeLabel string        `json:"time_label"`
	Duration  string        `json:"duration"`
	Event     EventResponse `json:"event"`
}

// TimelineResponse 是时间线详情。
type TimelineResponse struct {
	ID          string                  `json:"id"`
	Name        string                  `json:"name"`
	Description string                  `json:"description"`
	Entries     []TimelineEntryResponse `json:"entries"`
}

// PlotArcResponse 是剧情弧对外表示。
type PlotArcResponse struct {
	ID             string    `json:"id"`
	OriginalWorkID string    `json:"original_work_id"`
	Type           string    `json:"type"`
	Title          string    `json:"title"`
	Summary        string    `json:"summary"`
	StartEventID   *string   `json:"start_event_id"`
	EndEventID     *string   `json:"end_event_id"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func toEventResponse(e domain.OriginalEvent) EventResponse {
	participants := e.Participants
	if participants == nil {
		participants = []string{}
	}
	return EventResponse{
		ID: e.ID, OriginalWorkID: e.OriginalWorkID, Title: e.Title, Description: e.Description,
		ChapterNo: e.ChapterNo, TimeOrder: e.TimeOrder, Participants: participants,
		LocationID: e.LocationID, LocationText: e.LocationText, Consequences: e.Consequences,
		Importance: e.Importance, Source: string(e.Source),
		CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt,
	}
}

func toPlotArcResponse(p domain.PlotArc) PlotArcResponse {
	return PlotArcResponse{
		ID: p.ID, OriginalWorkID: p.OriginalWorkID, Type: string(p.Type), Title: p.Title,
		Summary: p.Summary, StartEventID: p.StartEventID, EndEventID: p.EndEventID,
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

func (req eventRequest) toInput() service.EventInput {
	return service.EventInput{
		Title: req.Title, Description: req.Description, ChapterNo: req.ChapterNo,
		TimeOrder: req.TimeOrder, Participants: req.Participants,
		LocationID: req.LocationID, LocationText: req.LocationText,
		Consequences: req.Consequences, Importance: req.Importance,
	}
}

// ---------- Handlers ----------

// listEvents 事件列表。
//
//	@Summary	事件列表
//	@Tags		events
//	@Produce	json
//	@Param		id			path		string	true	"原著 ID"
//	@Param		page		query		int		false	"页码"
//	@Param		page_size	query		int		false	"每页条数"
//	@Param		keyword		query		string	false	"标题/描述关键字"
//	@Success	200			{object}	Envelope
//	@Router		/original/{id}/events [get]
func (s *Server) listEvents(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	page := parseInt(c.Query("page"), 1)
	pageSize := parseInt(c.Query("page_size"), 100)
	items, total, err := s.events.ListEvents(c.Request.Context(), c.Param("id"), page, pageSize, c.Query("keyword"))
	if err != nil {
		s.failFromError(c, err)
		return
	}
	resp := EventListResponse{
		Items: make([]EventResponse, 0, len(items)), Total: total, Page: page, PageSize: pageSize,
	}
	for _, it := range items {
		resp.Items = append(resp.Items, toEventResponse(it))
	}
	OK(c, resp)
}

// createEvent 新增事件。
//
//	@Summary		新增事件
//	@Description	participants 与 location_id 必须属于同一部原著，否则 400。
//	@Tags			events
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string			true	"原著 ID"
//	@Param			body	body		eventRequest	true	"事件"
//	@Success		201		{object}	Envelope
//	@Failure		400		{object}	Envelope
//	@Router			/original/{id}/events [post]
func (s *Server) createEvent(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	var req eventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "请求参数不合法: "+err.Error(), nil)
		return
	}
	created, err := s.events.CreateEvent(c.Request.Context(), c.Param("id"), req.toInput())
	if err != nil {
		s.failFromError(c, err)
		return
	}
	Created(c, toEventResponse(*created))
}

// getEvent 事件详情。
//
//	@Summary	事件详情
//	@Tags		events
//	@Produce	json
//	@Param		id	path		string	true	"事件 ID"
//	@Success	200	{object}	Envelope
//	@Failure	404	{object}	Envelope
//	@Router		/events/{id} [get]
func (s *Server) getEvent(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	event, err := s.events.GetEventByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		s.failFromError(c, err)
		return
	}
	OK(c, toEventResponse(*event))
}

// updateEvent 更新事件。
//
//	@Summary	更新事件
//	@Tags		events
//	@Accept		json
//	@Produce	json
//	@Param		id		path		string			true	"事件 ID"
//	@Param		body	body		eventRequest	true	"事件"
//	@Success	200		{object}	Envelope
//	@Failure	400		{object}	Envelope
//	@Failure	404		{object}	Envelope
//	@Router		/events/{id} [put]
func (s *Server) updateEvent(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	var req eventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "请求参数不合法: "+err.Error(), nil)
		return
	}
	updated, err := s.events.UpdateEvent(c.Request.Context(), c.Param("id"), req.toInput())
	if err != nil {
		s.failFromError(c, err)
		return
	}
	OK(c, toEventResponse(*updated))
}

// deleteEvent 删除事件。
//
//	@Summary		删除事件
//	@Description	同时移除该事件在时间线上的条目。
//	@Tags			events
//	@Produce		json
//	@Param			id	path		string	true	"事件 ID"
//	@Success		200	{object}	Envelope
//	@Failure		404	{object}	Envelope
//	@Router			/events/{id} [delete]
func (s *Server) deleteEvent(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	if err := s.events.DeleteEvent(c.Request.Context(), c.Param("id")); err != nil {
		s.failFromError(c, err)
		return
	}
	OK(c, gin.H{"deleted": true})
}

// getTimeline 时间线详情。
//
//	@Summary		时间线详情
//	@Description	按顺序返回时间线条目（含事件本体）；时间线不存在时自动创建空时间线。
//	@Tags			timeline
//	@Produce		json
//	@Param			id	path		string	true	"原著 ID"
//	@Success		200	{object}	Envelope
//	@Failure		404	{object}	Envelope
//	@Router			/original/{id}/timeline [get]
func (s *Server) getTimeline(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	detail, err := s.events.GetTimeline(c.Request.Context(), c.Param("id"))
	if err != nil {
		s.failFromError(c, err)
		return
	}
	OK(c, toTimelineResponse(*detail))
}

// setTimelineOrder 设置时间线顺序。
//
//	@Summary		设置时间线顺序
//	@Description	整体替换时间线条目（幂等）：传入的顺序即最终顺序，未传入的事件会被移出时间线。
//	@Tags			timeline
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string					true	"原著 ID"
//	@Param			body	body		timelineOrderRequest	true	"有序条目"
//	@Success		200		{object}	Envelope
//	@Failure		400		{object}	Envelope
//	@Router			/original/{id}/timeline [put]
func (s *Server) setTimelineOrder(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	var req timelineOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "请求参数不合法: "+err.Error(), nil)
		return
	}
	items := make([]service.TimelineItemInput, 0, len(req.Items))
	for _, it := range req.Items {
		items = append(items, service.TimelineItemInput{
			EventID: it.EventID, TimeLabel: it.TimeLabel, Duration: it.Duration,
		})
	}
	detail, err := s.events.SetTimelineOrder(c.Request.Context(), c.Param("id"), items)
	if err != nil {
		s.failFromError(c, err)
		return
	}
	OK(c, toTimelineResponse(*detail))
}

// listPlotArcs 剧情弧列表。
//
//	@Summary	剧情弧列表
//	@Tags		plot
//	@Produce	json
//	@Param		id	path		string	true	"原著 ID"
//	@Success	200	{object}	Envelope
//	@Router		/original/{id}/plot-arcs [get]
func (s *Server) listPlotArcs(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	items, err := s.events.ListPlotArcs(c.Request.Context(), c.Param("id"))
	if err != nil {
		s.failFromError(c, err)
		return
	}
	resp := make([]PlotArcResponse, 0, len(items))
	for _, it := range items {
		resp = append(resp, toPlotArcResponse(it))
	}
	OK(c, gin.H{"items": resp, "total": len(resp)})
}

// createPlotArc 新增剧情弧。
//
//	@Summary	新增剧情弧
//	@Tags		plot
//	@Accept		json
//	@Produce	json
//	@Param		id		path		string			true	"原著 ID"
//	@Param		body	body		plotArcRequest	true	"剧情弧"
//	@Success	201		{object}	Envelope
//	@Failure	400		{object}	Envelope
//	@Router		/original/{id}/plot-arcs [post]
func (s *Server) createPlotArc(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	var req plotArcRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "请求参数不合法: "+err.Error(), nil)
		return
	}
	created, err := s.events.CreatePlotArc(c.Request.Context(), c.Param("id"), service.PlotArcInput{
		Type: domain.PlotArcType(req.Type), Title: req.Title, Summary: req.Summary,
		StartEventID: req.StartEventID, EndEventID: req.EndEventID,
	})
	if err != nil {
		s.failFromError(c, err)
		return
	}
	Created(c, toPlotArcResponse(*created))
}

// updatePlotArc 更新剧情弧。
//
//	@Summary	更新剧情弧
//	@Tags		plot
//	@Accept		json
//	@Produce	json
//	@Param		id		path		string			true	"剧情弧 ID"
//	@Param		body	body		plotArcRequest	true	"剧情弧"
//	@Success	200		{object}	Envelope
//	@Failure	404		{object}	Envelope
//	@Router		/plot-arcs/{id} [put]
func (s *Server) updatePlotArc(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	var req plotArcRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "请求参数不合法: "+err.Error(), nil)
		return
	}
	updated, err := s.events.UpdatePlotArc(c.Request.Context(), c.Param("id"), service.PlotArcInput{
		Type: domain.PlotArcType(req.Type), Title: req.Title, Summary: req.Summary,
		StartEventID: req.StartEventID, EndEventID: req.EndEventID,
	})
	if err != nil {
		s.failFromError(c, err)
		return
	}
	OK(c, toPlotArcResponse(*updated))
}

// deletePlotArc 删除剧情弧。
//
//	@Summary	删除剧情弧
//	@Tags		plot
//	@Produce	json
//	@Param		id	path		string	true	"剧情弧 ID"
//	@Success	200	{object}	Envelope
//	@Failure	404	{object}	Envelope
//	@Router		/plot-arcs/{id} [delete]
func (s *Server) deletePlotArc(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	if err := s.events.DeletePlotArc(c.Request.Context(), c.Param("id")); err != nil {
		s.failFromError(c, err)
		return
	}
	OK(c, gin.H{"deleted": true})
}

func toTimelineResponse(detail service.TimelineDetail) TimelineResponse {
	resp := TimelineResponse{
		ID: detail.Timeline.ID, Name: detail.Timeline.Name, Description: detail.Timeline.Description,
		Entries: make([]TimelineEntryResponse, 0, len(detail.Entries)),
	}
	for _, e := range detail.Entries {
		resp.Entries = append(resp.Entries, TimelineEntryResponse{
			Sequence: e.Sequence, TimeLabel: e.TimeLabel, Duration: e.Duration,
			Event: toEventResponse(e.Event),
		})
	}
	return resp
}
