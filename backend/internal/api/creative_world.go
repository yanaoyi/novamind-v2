package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/yanaoyi/novamindv2/backend/internal/domain"
	"github.com/yanaoyi/novamindv2/backend/internal/service"
)

// ---------- 请求 DTO ----------

type inheritWorldRequest struct {
	Mode string `json:"mode" binding:"required,oneof=FULL PARTIAL MODIFIED NEW" example:"PARTIAL"`
}

type updateCreativeWorldRequest struct {
	Name            *string `json:"name"`
	Description     *string `json:"description"`
	InheritanceMode *string `json:"inheritance_mode" binding:"omitempty,oneof=FULL PARTIAL MODIFIED NEW"`
}

type creativeWorldRuleRequest struct {
	Category     string  `json:"category" example:"社会规则"`
	Name         string  `json:"name" binding:"required" example:"老城区的人情规则"`
	Description  string  `json:"description"`
	Importance   int     `json:"importance" example:"4"`
	Status       string  `json:"status" binding:"omitempty,oneof=INHERITED MODIFIED REMOVED NEW"`
	SourceRuleID *string `json:"source_rule_id"`
}

type divergenceRequest struct {
	OriginalChapterID *string `json:"original_chapter_id"`
	OriginalEventID   *string `json:"original_event_id"`
	TimeLabel         string  `json:"time_label" example:"三年前·梅雨季"`
	Description       string  `json:"description" example:"从母亲意外之后改写"`
}

type creativeTimelineItemRequest struct {
	SourceOriginalEventID *string `json:"source_original_event_id"`
	Status                string  `json:"status" binding:"omitempty,oneof=INHERITED MODIFIED NEW REMOVED"`
	TimeLabel             string  `json:"time_label"`
	Title                 string  `json:"title" binding:"required"`
	Description           string  `json:"description"`
}

type setCreativeTimelineRequest struct {
	Items []creativeTimelineItemRequest `json:"items" binding:"required"`
}

// ---------- 响应 DTO ----------

// CreativeWorldRuleResponse 是二创世界规则对外表示。
type CreativeWorldRuleResponse struct {
	ID           string  `json:"id"`
	SourceRuleID *string `json:"source_rule_id"`
	Status       string  `json:"status"`
	Category     string  `json:"category"`
	Name         string  `json:"name"`
	Description  string  `json:"description"`
	Importance   int     `json:"importance"`
}

// CreativeWorldResponse 是二创世界对外表示。
type CreativeWorldResponse struct {
	ID              *string                     `json:"id"`
	CreativeWorkID  string                      `json:"creative_work_id"`
	SourceWorldID   *string                     `json:"source_world_id"`
	InheritanceMode string                      `json:"inheritance_mode"`
	Name            string                      `json:"name"`
	Description     string                      `json:"description"`
	InheritedCount  int                         `json:"inherited_count"`
	ModifiedCount   int                         `json:"modified_count"`
	RemovedCount    int                         `json:"removed_count"`
	NewCount        int                         `json:"new_count"`
	Rules           []CreativeWorldRuleResponse `json:"rules"`
	CreatedAt       *time.Time                  `json:"created_at"`
	UpdatedAt       *time.Time                  `json:"updated_at"`
}

// DivergenceResponse 是分叉点对外表示。
type DivergenceResponse struct {
	ID                string    `json:"id"`
	CreativeWorkID    string    `json:"creative_work_id"`
	OriginalChapterID *string   `json:"original_chapter_id"`
	OriginalEventID   *string   `json:"original_event_id"`
	TimeLabel         string    `json:"time_label"`
	Description       string    `json:"description"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// CreativeTimelineEventResponse 是二创时间线事件对外表示。
type CreativeTimelineEventResponse struct {
	ID                    string  `json:"id"`
	SourceOriginalEventID *string `json:"source_original_event_id"`
	Status                string  `json:"status"`
	Sequence              int     `json:"sequence"`
	TimeLabel             string  `json:"time_label"`
	Title                 string  `json:"title"`
	Description           string  `json:"description"`
}

func toCreativeWorldResponse(detail service.WorldDetail) CreativeWorldResponse {
	// 世界还没建时 detail.World 为 nil —— 返回空壳而不是崩掉
	if detail.World == nil {
		return CreativeWorldResponse{
			CreativeWorkID:  detail.CreativeWorkID,
			InheritanceMode: "NEW",
			Rules:           []CreativeWorldRuleResponse{},
		}
	}
	resp := CreativeWorldResponse{
		CreativeWorkID: detail.World.CreativeWorkID,
		Rules:          make([]CreativeWorldRuleResponse, 0, len(detail.Rules)),
	}
	id := detail.World.ID
	resp.ID = &id
	resp.SourceWorldID = detail.World.SourceWorldID
	resp.InheritanceMode = string(detail.World.InheritanceMode)
	resp.Name = detail.World.Name
	resp.Description = detail.World.Description
	resp.CreatedAt = &detail.World.CreatedAt
	resp.UpdatedAt = &detail.World.UpdatedAt
	for _, r := range detail.Rules {
		switch r.Status {
		case domain.RuleInherited:
			resp.InheritedCount++
		case domain.RuleModified:
			resp.ModifiedCount++
		case domain.RuleRemoved:
			resp.RemovedCount++
		case domain.RuleNew:
			resp.NewCount++
		}
		resp.Rules = append(resp.Rules, CreativeWorldRuleResponse{
			ID: r.ID, SourceRuleID: r.SourceRuleID, Status: string(r.Status),
			Category: r.Category, Name: r.Name, Description: r.Description, Importance: r.Importance,
		})
	}
	return resp
}

func toDivergenceResponse(d domain.DivergencePoint) DivergenceResponse {
	return DivergenceResponse{
		ID: d.ID, CreativeWorkID: d.CreativeWorkID,
		OriginalChapterID: d.OriginalChapterID, OriginalEventID: d.OriginalEventID,
		TimeLabel: d.TimeLabel, Description: d.Description,
		CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt,
	}
}

func toCreativeTimelineResponse(events []domain.CreativeTimelineEvent) []CreativeTimelineEventResponse {
	out := make([]CreativeTimelineEventResponse, 0, len(events))
	for _, e := range events {
		out = append(out, CreativeTimelineEventResponse{
			ID: e.ID, SourceOriginalEventID: e.SourceOriginalEventID, Status: string(e.Status),
			Sequence: e.Sequence, TimeLabel: e.TimeLabel, Title: e.Title, Description: e.Description,
		})
	}
	return out
}

// ---------- Handlers ----------

// getCreativeWorld 二创世界详情（含规则与各状态计数）。
//
//	@Summary	二创世界详情
//	@Description	还没建世界时返回空壳（id 为 null），便于前端直接渲染"去继承"入口。
//	@Tags		creative
//	@Produce	json
//	@Param		id	path		string	true	"二创作品 ID"
//	@Success	200	{object}	Envelope
//	@Router		/creative/{id}/world [get]
func (s *Server) getCreativeWorld(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	detail, err := s.creative.GetWorldDetail(c.Request.Context(), c.Param("id"))
	if err != nil {
		s.failFromError(c, err)
		return
	}
	OK(c, toCreativeWorldResponse(*detail))
}

// inheritCreativeWorld 从原著世界继承。
//
//	@Summary		继承原著世界
//	@Description	FULL/PARTIAL 会把原著规则整套带过来（INHERITED），作者再逐条改；MODIFIED/NEW 先建空世界。
//	@Tags			creative
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string					true	"二创作品 ID"
//	@Param			body	body		inheritWorldRequest		true	"继承模式"
//	@Success		200		{object}	Envelope
//	@Failure		400		{object}	Envelope
//	@Router			/creative/{id}/world/inherit [post]
func (s *Server) inheritCreativeWorld(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	var req inheritWorldRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "请求参数不合法: "+err.Error(), nil)
		return
	}
	detail, err := s.creative.InheritWorld(c.Request.Context(), c.Param("id"), domain.WorldInheritanceMode(req.Mode))
	if err != nil {
		s.failFromError(c, err)
		return
	}
	OK(c, toCreativeWorldResponse(*detail))
}

// updateCreativeWorld 更新二创世界。
//
//	@Summary	更新二创世界
//	@Tags		creative
//	@Accept		json
//	@Produce	json
//	@Param		id		path		string						true	"二创作品 ID"
//	@Param		body	body		updateCreativeWorldRequest	true	"待更新字段"
//	@Success	200		{object}	Envelope
//	@Failure	404		{object}	Envelope
//	@Router		/creative/{id}/world [put]
func (s *Server) updateCreativeWorld(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	var req updateCreativeWorldRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "请求参数不合法: "+err.Error(), nil)
		return
	}
	in := service.UpdateWorldInput{Name: req.Name, Description: req.Description}
	if req.InheritanceMode != nil {
		mode := domain.WorldInheritanceMode(*req.InheritanceMode)
		in.InheritanceMode = &mode
	}
	detail, err := s.creative.UpdateWorld(c.Request.Context(), c.Param("id"), in)
	if err != nil {
		s.failFromError(c, err)
		return
	}
	OK(c, toCreativeWorldResponse(*detail))
}

// createCreativeWorldRule 新增二创世界规则。
//
//	@Summary		新增二创世界规则
//	@Description	不带 source_rule_id 视为 NEW；带上则视为从该原著规则改出来的 MODIFIED。
//	@Tags			creative
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string						true	"二创作品 ID"
//	@Param			body	body		creativeWorldRuleRequest	true	"规则"
//	@Success		201		{object}	Envelope
//	@Failure		400		{object}	Envelope
//	@Failure		409		{object}	Envelope
//	@Router			/creative/{id}/world/rules [post]
func (s *Server) createCreativeWorldRule(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	var req creativeWorldRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "请求参数不合法: "+err.Error(), nil)
		return
	}
	rule, err := s.creative.CreateWorldRule(c.Request.Context(), c.Param("id"), service.WorldRuleInput{
		Category: req.Category, Name: req.Name, Description: req.Description,
		Importance: req.Importance, Status: domain.CreativeWorldRuleStatus(req.Status),
		SourceRuleID: req.SourceRuleID,
	})
	if err != nil {
		s.failFromError(c, err)
		return
	}
	Created(c, gin.H{
		"id": rule.ID, "status": string(rule.Status), "name": rule.Name,
		"category": rule.Category, "description": rule.Description, "importance": rule.Importance,
		"source_rule_id": rule.SourceRuleID,
	})
}

// updateCreativeWorldRule 更新二创世界规则。
//
//	@Summary	更新二创世界规则
//	@Tags		creative
//	@Accept		json
//	@Produce	json
//	@Param		id		path		string						true	"规则 ID"
//	@Param		body	body		creativeWorldRuleRequest	true	"规则"
//	@Success	200		{object}	Envelope
//	@Failure	404		{object}	Envelope
//	@Router		/creative-world-rules/{id} [put]
func (s *Server) updateCreativeWorldRule(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	var req creativeWorldRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "请求参数不合法: "+err.Error(), nil)
		return
	}
	rule, err := s.creative.UpdateWorldRule(c.Request.Context(), c.Param("id"), service.WorldRuleInput{
		Category: req.Category, Name: req.Name, Description: req.Description,
		Importance: req.Importance, Status: domain.CreativeWorldRuleStatus(req.Status),
	})
	if err != nil {
		s.failFromError(c, err)
		return
	}
	OK(c, gin.H{
		"id": rule.ID, "status": string(rule.Status), "name": rule.Name,
		"category": rule.Category, "description": rule.Description, "importance": rule.Importance,
	})
}

// deleteCreativeWorldRule 在二创里删掉一条规则。
//
//	@Summary		删除二创世界规则
//	@Description	继承来的规则标记为 REMOVED（保留可追溯性并写映射）；纯新增的直接删除。
//	@Tags			creative
//	@Produce		json
//	@Param			id	path		string	true	"规则 ID"
//	@Success		200	{object}	Envelope
//	@Failure		404	{object}	Envelope
//	@Router			/creative-world-rules/{id} [delete]
func (s *Server) deleteCreativeWorldRule(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	if err := s.creative.RemoveWorldRule(c.Request.Context(), c.Param("id")); err != nil {
		s.failFromError(c, err)
		return
	}
	OK(c, gin.H{"deleted": true})
}

// getDivergence 分叉点详情。
//
//	@Summary	分叉点详情
//	@Tags		creative
//	@Produce	json
//	@Param		id	path		string	true	"二创作品 ID"
//	@Success	200	{object}	Envelope
//	@Router		/creative/{id}/divergence [get]
func (s *Server) getDivergence(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	point, err := s.creative.GetDivergence(c.Request.Context(), c.Param("id"))
	if err != nil {
		s.failFromError(c, err)
		return
	}
	if point == nil {
		OK(c, nil)
		return
	}
	OK(c, toDivergenceResponse(*point))
}

// setDivergence 设置分叉点。
//
//	@Summary		设置分叉点
//	@Description	必须指向该二创作品所依据原著的事件（或章节），否则 400。
//	@Tags			creative
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string				true	"二创作品 ID"
//	@Param			body	body		divergenceRequest	true	"分叉点"
//	@Success		200		{object}	Envelope
//	@Failure		400		{object}	Envelope
//	@Router			/creative/{id}/divergence [put]
func (s *Server) setDivergence(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	var req divergenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "请求参数不合法: "+err.Error(), nil)
		return
	}
	point, err := s.creative.SetDivergence(c.Request.Context(), c.Param("id"), service.DivergenceInput{
		OriginalChapterID: req.OriginalChapterID, OriginalEventID: req.OriginalEventID,
		TimeLabel: req.TimeLabel, Description: req.Description,
	})
	if err != nil {
		s.failFromError(c, err)
		return
	}
	OK(c, toDivergenceResponse(*point))
}

// getCreativeTimeline 二创时间线。
//
//	@Summary	二创时间线
//	@Tags		creative
//	@Produce	json
//	@Param		id	path		string	true	"二创作品 ID"
//	@Success	200	{object}	Envelope
//	@Router		/creative/{id}/timeline [get]
func (s *Server) getCreativeTimeline(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	events, err := s.creative.GetTimeline(c.Request.Context(), c.Param("id"))
	if err != nil {
		s.failFromError(c, err)
		return
	}
	OK(c, gin.H{"items": toCreativeTimelineResponse(events), "total": len(events)})
}

// buildCreativeTimeline 自动构建二创时间线。
//
//	@Summary		自动构建二创时间线
//	@Description	把分叉点（含）之前的原著事件按序继承过来，并保留作者已有的二创新事件。
//	@Tags			creative
//	@Produce		json
//	@Param			id	path		string	true	"二创作品 ID"
//	@Success		200	{object}	Envelope
//	@Router			/creative/{id}/timeline/build [post]
func (s *Server) buildCreativeTimeline(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	events, err := s.creative.BuildTimeline(c.Request.Context(), c.Param("id"))
	if err != nil {
		s.failFromError(c, err)
		return
	}
	OK(c, gin.H{"items": toCreativeTimelineResponse(events), "total": len(events)})
}

// setCreativeTimeline 手工编排二创时间线。
//
//	@Summary		保存二创时间线
//	@Description	整体替换（幂等）：传入顺序即最终顺序。
//	@Tags			creative
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string						true	"二创作品 ID"
//	@Param			body	body		setCreativeTimelineRequest	true	"有序事件"
//	@Success		200		{object}	Envelope
//	@Failure		400		{object}	Envelope
//	@Router			/creative/{id}/timeline [put]
func (s *Server) setCreativeTimeline(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	var req setCreativeTimelineRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "请求参数不合法: "+err.Error(), nil)
		return
	}
	items := make([]service.CreativeTimelineItemInput, 0, len(req.Items))
	for _, it := range req.Items {
		items = append(items, service.CreativeTimelineItemInput{
			SourceOriginalEventID: it.SourceOriginalEventID,
			Status:                domain.CreativeTimelineEventStatus(it.Status),
			TimeLabel:             it.TimeLabel, Title: it.Title, Description: it.Description,
		})
	}
	events, err := s.creative.SetTimeline(c.Request.Context(), c.Param("id"), items)
	if err != nil {
		s.failFromError(c, err)
		return
	}
	OK(c, gin.H{"items": toCreativeTimelineResponse(events), "total": len(events)})
}
