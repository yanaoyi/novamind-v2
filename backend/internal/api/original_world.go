package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/yanaoyi/novamindv2/backend/internal/domain"
	"github.com/yanaoyi/novamindv2/backend/internal/service"
)

// ---------- 请求 DTO ----------

type upsertWorldRequest struct {
	Name        string `json:"name" example:"九州"`
	Description string `json:"description" example:"架空大陆，灵力为唯一超自然力量"`
}

type worldRuleRequest struct {
	Category    string `json:"category" example:"力量体系"`
	Name        string `json:"name" binding:"required" example:"灵力不可凭空产生"`
	Description string `json:"description"`
	Importance  int    `json:"importance" example:"5"`
}

type locationRequest struct {
	Name             string  `json:"name" binding:"required" example:"青云城"`
	Type             string  `json:"type" example:"城市"`
	Description      string  `json:"description"`
	ParentLocationID *string `json:"parent_location_id"`
}

type factionRequest struct {
	Name          string `json:"name" binding:"required" example:"天枢阁"`
	Type          string `json:"type" example:"宗门"`
	Description   string `json:"description"`
	Goals         string `json:"goals" example:"垄断灵矿"`
	Relationships string `json:"relationships" example:"与玄冥教敌对"`
}

// ---------- 响应 DTO ----------

// WorldResponse 是世界观对外表示。
type WorldResponse struct {
	ID             string    `json:"id"`
	OriginalWorkID string    `json:"original_work_id"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	RuleCount      int       `json:"rule_count"`
	LocationCount  int       `json:"location_count"`
	FactionCount   int       `json:"faction_count"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// WorldRuleResponse 是世界规则对外表示。
type WorldRuleResponse struct {
	ID          string    `json:"id"`
	WorldID     string    `json:"world_id"`
	Category    string    `json:"category"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Importance  int       `json:"importance"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// LocationResponse 是地点对外表示。
type LocationResponse struct {
	ID               string    `json:"id"`
	WorldID          string    `json:"world_id"`
	Name             string    `json:"name"`
	Type             string    `json:"type"`
	Description      string    `json:"description"`
	ParentLocationID *string   `json:"parent_location_id"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// FactionResponse 是势力对外表示。
type FactionResponse struct {
	ID            string    `json:"id"`
	WorldID       string    `json:"world_id"`
	Name          string    `json:"name"`
	Type          string    `json:"type"`
	Description   string    `json:"description"`
	Goals         string    `json:"goals"`
	Relationships string    `json:"relationships"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func toWorldResponse(summary domain.WorldSummary) WorldResponse {
	return WorldResponse{
		ID:             summary.World.ID,
		OriginalWorkID: summary.World.OriginalWorkID,
		Name:           summary.World.Name,
		Description:    summary.World.Description,
		RuleCount:      summary.Rules,
		LocationCount:  summary.Locations,
		FactionCount:   summary.Factions,
		CreatedAt:      summary.World.CreatedAt,
		UpdatedAt:      summary.World.UpdatedAt,
	}
}

func toWorldRuleResponse(r domain.WorldRule) WorldRuleResponse {
	return WorldRuleResponse{
		ID: r.ID, WorldID: r.WorldID, Category: r.Category, Name: r.Name,
		Description: r.Description, Importance: r.Importance,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

func toLocationResponse(l domain.Location) LocationResponse {
	return LocationResponse{
		ID: l.ID, WorldID: l.WorldID, Name: l.Name, Type: l.Type,
		Description: l.Description, ParentLocationID: l.ParentLocationID,
		CreatedAt: l.CreatedAt, UpdatedAt: l.UpdatedAt,
	}
}

func toFactionResponse(f domain.Faction) FactionResponse {
	return FactionResponse{
		ID: f.ID, WorldID: f.WorldID, Name: f.Name, Type: f.Type,
		Description: f.Description, Goals: f.Goals, Relationships: f.Relationships,
		CreatedAt: f.CreatedAt, UpdatedAt: f.UpdatedAt,
	}
}

// ---------- Handlers ----------

// getWorld 世界观概览。
//
//	@Summary		世界观概览
//	@Description	返回世界设定与规则/地点/势力数量；尚未创建时返回 404。
//	@Tags			world
//	@Produce		json
//	@Param			id	path		string	true	"原著 ID"
//	@Success		200	{object}	Envelope
//	@Failure		404	{object}	Envelope
//	@Router			/original/{id}/world [get]
func (s *Server) getWorld(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	summary, err := s.worlds.GetSummary(c.Request.Context(), c.Param("id"))
	if err != nil {
		s.failFromError(c, err)
		return
	}
	OK(c, toWorldResponse(*summary))
}

// upsertWorld 创建或更新世界观。
//
//	@Summary		创建/更新世界观
//	@Description	一个原著只有一个世界；重复调用为更新。
//	@Tags			world
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string				true	"原著 ID"
//	@Param			body	body		upsertWorldRequest	true	"世界设定"
//	@Success		200		{object}	Envelope
//	@Failure		404		{object}	Envelope
//	@Router			/original/{id}/world [put]
func (s *Server) upsertWorld(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	var req upsertWorldRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "请求参数不合法: "+err.Error(), nil)
		return
	}
	workID := c.Param("id")
	if _, err := s.worlds.UpsertWorld(c.Request.Context(), workID, req.Name, req.Description); err != nil {
		s.failFromError(c, err)
		return
	}
	summary, err := s.worlds.GetSummary(c.Request.Context(), workID)
	if err != nil {
		s.failFromError(c, err)
		return
	}
	OK(c, toWorldResponse(*summary))
}

// listWorldRules 规则列表。
//
//	@Summary	世界规则列表
//	@Tags		world
//	@Produce	json
//	@Param		id	path		string	true	"原著 ID"
//	@Success	200	{object}	Envelope
//	@Router		/original/{id}/rules [get]
func (s *Server) listWorldRules(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	items, err := s.worlds.ListRules(c.Request.Context(), c.Param("id"))
	if err != nil {
		s.failFromError(c, err)
		return
	}
	resp := make([]WorldRuleResponse, 0, len(items))
	for _, it := range items {
		resp = append(resp, toWorldRuleResponse(it))
	}
	OK(c, gin.H{"items": resp, "total": len(resp)})
}

// createWorldRule 新增规则。
//
//	@Summary	新增世界规则
//	@Tags		world
//	@Accept		json
//	@Produce	json
//	@Param		id		path		string				true	"原著 ID"
//	@Param		body	body		worldRuleRequest	true	"规则"
//	@Success	201		{object}	Envelope
//	@Failure	400		{object}	Envelope
//	@Failure	409		{object}	Envelope
//	@Router		/original/{id}/rules [post]
func (s *Server) createWorldRule(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	var req worldRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "请求参数不合法: "+err.Error(), nil)
		return
	}
	created, err := s.worlds.CreateRule(c.Request.Context(), c.Param("id"), service.RuleInput{
		Category: req.Category, Name: req.Name, Description: req.Description, Importance: req.Importance,
	})
	if err != nil {
		s.failFromError(c, err)
		return
	}
	Created(c, toWorldRuleResponse(*created))
}

// updateWorldRule 更新规则。
//
//	@Summary	更新世界规则
//	@Tags		world
//	@Accept		json
//	@Produce	json
//	@Param		id		path		string				true	"规则 ID"
//	@Param		body	body		worldRuleRequest	true	"规则"
//	@Success	200		{object}	Envelope
//	@Failure	404		{object}	Envelope
//	@Router		/world-rules/{id} [put]
func (s *Server) updateWorldRule(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	var req worldRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "请求参数不合法: "+err.Error(), nil)
		return
	}
	updated, err := s.worlds.UpdateRule(c.Request.Context(), c.Param("id"), service.RuleInput{
		Category: req.Category, Name: req.Name, Description: req.Description, Importance: req.Importance,
	})
	if err != nil {
		s.failFromError(c, err)
		return
	}
	OK(c, toWorldRuleResponse(*updated))
}

// deleteWorldRule 删除规则。
//
//	@Summary	删除世界规则
//	@Tags		world
//	@Produce	json
//	@Param		id	path		string	true	"规则 ID"
//	@Success	200	{object}	Envelope
//	@Failure	404	{object}	Envelope
//	@Router		/world-rules/{id} [delete]
func (s *Server) deleteWorldRule(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	if err := s.worlds.DeleteRule(c.Request.Context(), c.Param("id")); err != nil {
		s.failFromError(c, err)
		return
	}
	OK(c, gin.H{"deleted": true})
}

// listLocations 地点列表。
//
//	@Summary	地点列表
//	@Tags		world
//	@Produce	json
//	@Param		id	path		string	true	"原著 ID"
//	@Success	200	{object}	Envelope
//	@Router		/original/{id}/locations [get]
func (s *Server) listLocations(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	items, err := s.worlds.ListLocations(c.Request.Context(), c.Param("id"))
	if err != nil {
		s.failFromError(c, err)
		return
	}
	resp := make([]LocationResponse, 0, len(items))
	for _, it := range items {
		resp = append(resp, toLocationResponse(it))
	}
	OK(c, gin.H{"items": resp, "total": len(resp)})
}

// createLocation 新增地点。
//
//	@Summary		新增地点
//	@Description	parent_location_id 可选，用于建立「大陆 → 国家 → 城市」层级；同世界内名称唯一。
//	@Tags			world
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string			true	"原著 ID"
//	@Param			body	body		locationRequest	true	"地点"
//	@Success		201		{object}	Envelope
//	@Failure		400		{object}	Envelope
//	@Router			/original/{id}/locations [post]
func (s *Server) createLocation(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	var req locationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "请求参数不合法: "+err.Error(), nil)
		return
	}
	created, err := s.worlds.CreateLocation(c.Request.Context(), c.Param("id"), service.LocationInput{
		Name: req.Name, Type: req.Type, Description: req.Description, ParentLocationID: req.ParentLocationID,
	})
	if err != nil {
		s.failFromError(c, err)
		return
	}
	Created(c, toLocationResponse(*created))
}

// updateLocation 更新地点。
//
//	@Summary		更新地点
//	@Description	修改上级地点时会校验同世界且不形成环。
//	@Tags			world
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string			true	"地点 ID"
//	@Param			body	body		locationRequest	true	"地点"
//	@Success		200		{object}	Envelope
//	@Failure		400		{object}	Envelope
//	@Failure		404		{object}	Envelope
//	@Router			/locations/{id} [put]
func (s *Server) updateLocation(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	var req locationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "请求参数不合法: "+err.Error(), nil)
		return
	}
	updated, err := s.worlds.UpdateLocation(c.Request.Context(), c.Param("id"), service.LocationInput{
		Name: req.Name, Type: req.Type, Description: req.Description, ParentLocationID: req.ParentLocationID,
	})
	if err != nil {
		s.failFromError(c, err)
		return
	}
	OK(c, toLocationResponse(*updated))
}

// deleteLocation 删除地点。
//
//	@Summary	删除地点
//	@Description	子地点的上级会被置空，不会产生悬挂引用。
//	@Tags		world
//	@Produce	json
//	@Param		id	path		string	true	"地点 ID"
//	@Success	200	{object}	Envelope
//	@Failure	404	{object}	Envelope
//	@Router		/locations/{id} [delete]
func (s *Server) deleteLocation(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	if err := s.worlds.DeleteLocation(c.Request.Context(), c.Param("id")); err != nil {
		s.failFromError(c, err)
		return
	}
	OK(c, gin.H{"deleted": true})
}

// listFactions 势力列表。
//
//	@Summary	势力列表
//	@Tags		world
//	@Produce	json
//	@Param		id	path		string	true	"原著 ID"
//	@Success	200	{object}	Envelope
//	@Router		/original/{id}/factions [get]
func (s *Server) listFactions(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	items, err := s.worlds.ListFactions(c.Request.Context(), c.Param("id"))
	if err != nil {
		s.failFromError(c, err)
		return
	}
	resp := make([]FactionResponse, 0, len(items))
	for _, it := range items {
		resp = append(resp, toFactionResponse(it))
	}
	OK(c, gin.H{"items": resp, "total": len(resp)})
}

// createFaction 新增势力。
//
//	@Summary	新增势力
//	@Tags		world
//	@Accept		json
//	@Produce	json
//	@Param		id		path		string			true	"原著 ID"
//	@Param		body	body		factionRequest	true	"势力"
//	@Success	201		{object}	Envelope
//	@Failure	400		{object}	Envelope
//	@Router		/original/{id}/factions [post]
func (s *Server) createFaction(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	var req factionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "请求参数不合法: "+err.Error(), nil)
		return
	}
	created, err := s.worlds.CreateFaction(c.Request.Context(), c.Param("id"), service.FactionInput{
		Name: req.Name, Type: req.Type, Description: req.Description,
		Goals: req.Goals, Relationships: req.Relationships,
	})
	if err != nil {
		s.failFromError(c, err)
		return
	}
	Created(c, toFactionResponse(*created))
}

// updateFaction 更新势力。
//
//	@Summary	更新势力
//	@Tags		world
//	@Accept		json
//	@Produce	json
//	@Param		id		path		string			true	"势力 ID"
//	@Param		body	body		factionRequest	true	"势力"
//	@Success	200		{object}	Envelope
//	@Failure	404		{object}	Envelope
//	@Router		/factions/{id} [put]
func (s *Server) updateFaction(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	var req factionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "请求参数不合法: "+err.Error(), nil)
		return
	}
	updated, err := s.worlds.UpdateFaction(c.Request.Context(), c.Param("id"), service.FactionInput{
		Name: req.Name, Type: req.Type, Description: req.Description,
		Goals: req.Goals, Relationships: req.Relationships,
	})
	if err != nil {
		s.failFromError(c, err)
		return
	}
	OK(c, toFactionResponse(*updated))
}

// deleteFaction 删除势力。
//
//	@Summary	删除势力
//	@Tags		world
//	@Produce	json
//	@Param		id	path		string	true	"势力 ID"
//	@Success	200	{object}	Envelope
//	@Failure	404	{object}	Envelope
//	@Router		/factions/{id} [delete]
func (s *Server) deleteFaction(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	if err := s.worlds.DeleteFaction(c.Request.Context(), c.Param("id")); err != nil {
		s.failFromError(c, err)
		return
	}
	OK(c, gin.H{"deleted": true})
}
