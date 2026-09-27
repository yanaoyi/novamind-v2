package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/yanaoyi/novamindv2/backend/internal/domain"
	"github.com/yanaoyi/novamindv2/backend/internal/service"
)

// ---------- 请求 DTO ----------

// characterRequest 同时用于创建与更新（更新为全量覆盖）。
type characterRequest struct {
	Name             string              `json:"name" binding:"required" example:"林默"`
	Aliases          []string            `json:"aliases" example:"小默,林先生"`
	Role             string              `json:"role" example:"主角"`
	Gender           string              `json:"gender" example:"女"`
	Age              string              `json:"age" example:"24"`
	Appearance       string              `json:"appearance"`
	Personality      string              `json:"personality"`
	Motivation       string              `json:"motivation"`
	Values           string              `json:"values"`
	Fears            string              `json:"fears"`
	Desires          string              `json:"desires"`
	BehaviorPatterns string              `json:"behavior_patterns"`
	SpeechStyle      string              `json:"speech_style"`
	Abilities        string              `json:"abilities"`
	FirstAppearance  string              `json:"first_appearance"`
	LastAppearance   string              `json:"last_appearance"`
	DNA              domain.CharacterDNA `json:"dna"`
	Importance       int                 `json:"importance" example:"5"`
	Notes            string              `json:"notes"`
}

type relationshipRequest struct {
	SourceCharacterID string `json:"source_character_id" binding:"required"`
	TargetCharacterID string `json:"target_character_id" binding:"required"`
	RelationType      string `json:"relation_type" binding:"required,oneof=family friend lover enemy mentor student colleague rival organization other"`
	Strength          int    `json:"strength" example:"80"`
	Description       string `json:"description"`
}

type updateRelationshipRequest struct {
	RelationType string `json:"relation_type" binding:"omitempty,oneof=family friend lover enemy mentor student colleague rival organization other"`
	Strength     int    `json:"strength" example:"80"`
	Description  string `json:"description"`
}

// ---------- 响应 DTO ----------

// CharacterResponse 是人物对外表示。
type CharacterResponse struct {
	ID               string              `json:"id"`
	OriginalWorkID   string              `json:"original_work_id"`
	Name             string              `json:"name"`
	Aliases          []string            `json:"aliases"`
	Role             string              `json:"role"`
	Gender           string              `json:"gender"`
	Age              string              `json:"age"`
	Appearance       string              `json:"appearance"`
	Personality      string              `json:"personality"`
	Motivation       string              `json:"motivation"`
	Values           string              `json:"values"`
	Fears            string              `json:"fears"`
	Desires          string              `json:"desires"`
	BehaviorPatterns string              `json:"behavior_patterns"`
	SpeechStyle      string              `json:"speech_style"`
	Abilities        string              `json:"abilities"`
	FirstAppearance  string              `json:"first_appearance"`
	LastAppearance   string              `json:"last_appearance"`
	DNA              domain.CharacterDNA `json:"dna"`
	Importance       int                 `json:"importance"`
	Source           string              `json:"source"`
	Notes            string              `json:"notes"`
	CreatedAt        time.Time           `json:"created_at"`
	UpdatedAt        time.Time           `json:"updated_at"`
}

// CharacterListResponse 是人物列表响应。
type CharacterListResponse struct {
	Items    []CharacterResponse `json:"items"`
	Total    int64               `json:"total"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"page_size"`
}

// RelationshipResponse 是人物关系对外表示。
type RelationshipResponse struct {
	ID                string    `json:"id"`
	OriginalWorkID    string    `json:"original_work_id"`
	SourceCharacterID string    `json:"source_character_id"`
	TargetCharacterID string    `json:"target_character_id"`
	RelationType      string    `json:"relation_type"`
	Strength          int       `json:"strength"`
	Description       string    `json:"description"`
	Source            string    `json:"source"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// RelationshipListResponse 是关系列表响应。
type RelationshipListResponse struct {
	Items []RelationshipResponse `json:"items"`
	Total int                    `json:"total"`
}

func toCharacterResponse(c domain.OriginalCharacter) CharacterResponse {
	aliases := c.Aliases
	if aliases == nil {
		aliases = []string{}
	}
	return CharacterResponse{
		ID:               c.ID,
		OriginalWorkID:   c.OriginalWorkID,
		Name:             c.Name,
		Aliases:          aliases,
		Role:             c.Role,
		Gender:           c.Gender,
		Age:              c.Age,
		Appearance:       c.Appearance,
		Personality:      c.Personality,
		Motivation:       c.Motivation,
		Values:           c.Values,
		Fears:            c.Fears,
		Desires:          c.Desires,
		BehaviorPatterns: c.BehaviorPatterns,
		SpeechStyle:      c.SpeechStyle,
		Abilities:        c.Abilities,
		FirstAppearance:  c.FirstAppearance,
		LastAppearance:   c.LastAppearance,
		DNA:              c.DNA,
		Importance:       c.Importance,
		Source:           string(c.Source),
		Notes:            c.Notes,
		CreatedAt:        c.CreatedAt,
		UpdatedAt:        c.UpdatedAt,
	}
}

func toRelationshipResponse(r domain.CharacterRelationship) RelationshipResponse {
	return RelationshipResponse{
		ID:                r.ID,
		OriginalWorkID:    r.OriginalWorkID,
		SourceCharacterID: r.SourceCharacterID,
		TargetCharacterID: r.TargetCharacterID,
		RelationType:      string(r.RelationType),
		Strength:          r.Strength,
		Description:       r.Description,
		Source:            string(r.Source),
		CreatedAt:         r.CreatedAt,
		UpdatedAt:         r.UpdatedAt,
	}
}

func (req characterRequest) toInput() service.CharacterInput {
	return service.CharacterInput{
		Name:             req.Name,
		Aliases:          req.Aliases,
		Role:             req.Role,
		Gender:           req.Gender,
		Age:              req.Age,
		Appearance:       req.Appearance,
		Personality:      req.Personality,
		Motivation:       req.Motivation,
		Values:           req.Values,
		Fears:            req.Fears,
		Desires:          req.Desires,
		BehaviorPatterns: req.BehaviorPatterns,
		SpeechStyle:      req.SpeechStyle,
		Abilities:        req.Abilities,
		FirstAppearance:  req.FirstAppearance,
		LastAppearance:   req.LastAppearance,
		DNA:              req.DNA,
		Importance:       req.Importance,
		Notes:            req.Notes,
	}
}

// ---------- Handlers ----------

// createCharacter 新增原著人物。
//
//	@Summary		新增人物
//	@Description	为原著新增人物；同原著内姓名唯一。dna 为人物 DNA（各维度 text + 0-100 权重）。
//	@Tags			characters
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string				true	"原著 ID"
//	@Param			body	body		characterRequest	true	"人物信息"
//	@Success		201		{object}	Envelope
//	@Failure		400		{object}	Envelope
//	@Failure		404		{object}	Envelope
//	@Failure		409		{object}	Envelope
//	@Router			/original/{id}/characters [post]
func (s *Server) createCharacter(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	var req characterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "请求参数不合法: "+err.Error(), nil)
		return
	}
	created, err := s.characters.CreateCharacter(c.Request.Context(), c.Param("id"), req.toInput())
	if err != nil {
		s.failFromError(c, err)
		return
	}
	Created(c, toCharacterResponse(*created))
}

// listCharacters 人物列表。
//
//	@Summary	人物列表
//	@Tags		characters
//	@Produce	json
//	@Param		id			path		string	true	"原著 ID"
//	@Param		page		query		int		false	"页码，默认 1"
//	@Param		page_size	query		int		false	"每页条数，默认 50，最大 200"
//	@Param		keyword		query		string	false	"按姓名/角色模糊搜索"
//	@Success	200			{object}	Envelope
//	@Failure	404			{object}	Envelope
//	@Router		/original/{id}/characters [get]
func (s *Server) listCharacters(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	page := parseInt(c.Query("page"), 1)
	pageSize := parseInt(c.Query("page_size"), 50)

	items, total, err := s.characters.ListCharacters(c.Request.Context(), c.Param("id"), page, pageSize, c.Query("keyword"))
	if err != nil {
		s.failFromError(c, err)
		return
	}
	resp := CharacterListResponse{
		Items:    make([]CharacterResponse, 0, len(items)),
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}
	for _, it := range items {
		resp.Items = append(resp.Items, toCharacterResponse(it))
	}
	OK(c, resp)
}

// getCharacter 人物详情。
//
//	@Summary	人物详情
//	@Tags		characters
//	@Produce	json
//	@Param		id	path		string	true	"人物 ID"
//	@Success	200	{object}	Envelope
//	@Failure	404	{object}	Envelope
//	@Router		/characters/{id} [get]
func (s *Server) getCharacter(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	found, err := s.characters.GetCharacter(c.Request.Context(), c.Param("id"))
	if err != nil {
		s.failFromError(c, err)
		return
	}
	OK(c, toCharacterResponse(*found))
}

// updateCharacter 更新人物（全量覆盖）。
//
//	@Summary		更新人物
//	@Description	全量覆盖式更新：未提交的字段会被置空，前端应回传完整对象。
//	@Tags			characters
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string				true	"人物 ID"
//	@Param			body	body		characterRequest	true	"人物信息（全量）"
//	@Success		200		{object}	Envelope
//	@Failure		400		{object}	Envelope
//	@Failure		404		{object}	Envelope
//	@Failure		409		{object}	Envelope
//	@Router			/characters/{id} [put]
func (s *Server) updateCharacter(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	var req characterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "请求参数不合法: "+err.Error(), nil)
		return
	}
	updated, err := s.characters.UpdateCharacter(c.Request.Context(), c.Param("id"), req.toInput())
	if err != nil {
		s.failFromError(c, err)
		return
	}
	OK(c, toCharacterResponse(*updated))
}

// deleteCharacter 删除人物。
//
//	@Summary		删除人物
//	@Description	软删除；与其相关的人物关系一并删除。
//	@Tags			characters
//	@Produce		json
//	@Param			id	path		string	true	"人物 ID"
//	@Success		200	{object}	Envelope
//	@Failure		404	{object}	Envelope
//	@Router			/characters/{id} [delete]
func (s *Server) deleteCharacter(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	if err := s.characters.DeleteCharacter(c.Request.Context(), c.Param("id")); err != nil {
		s.failFromError(c, err)
		return
	}
	OK(c, gin.H{"deleted": true})
}

// createRelationship 新增人物关系。
//
//	@Summary		新增人物关系
//	@Tags			characters
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string				true	"原著 ID"
//	@Param			body	body		relationshipRequest	true	"关系信息"
//	@Success		201		{object}	Envelope
//	@Failure		400		{object}	Envelope
//	@Failure		404		{object}	Envelope
//	@Failure		409		{object}	Envelope
//	@Router			/original/{id}/relationships [post]
func (s *Server) createRelationship(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	var req relationshipRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "请求参数不合法: "+err.Error(), nil)
		return
	}
	created, err := s.characters.CreateRelationship(c.Request.Context(), c.Param("id"), service.RelationshipInput{
		SourceCharacterID: req.SourceCharacterID,
		TargetCharacterID: req.TargetCharacterID,
		RelationType:      domain.RelationType(req.RelationType),
		Strength:          req.Strength,
		Description:       req.Description,
	})
	if err != nil {
		s.failFromError(c, err)
		return
	}
	Created(c, toRelationshipResponse(*created))
}

// listRelationships 人物关系列表。
//
//	@Summary	人物关系列表
//	@Tags		characters
//	@Produce	json
//	@Param		id	path		string	true	"原著 ID"
//	@Success	200	{object}	Envelope
//	@Failure	404	{object}	Envelope
//	@Router		/original/{id}/relationships [get]
func (s *Server) listRelationships(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	items, err := s.characters.ListRelationships(c.Request.Context(), c.Param("id"))
	if err != nil {
		s.failFromError(c, err)
		return
	}
	resp := RelationshipListResponse{Items: make([]RelationshipResponse, 0, len(items)), Total: len(items)}
	for _, it := range items {
		resp.Items = append(resp.Items, toRelationshipResponse(it))
	}
	OK(c, resp)
}

// updateRelationship 更新人物关系。
//
//	@Summary	更新人物关系
//	@Tags		characters
//	@Accept		json
//	@Produce	json
//	@Param		id		path		string						true	"关系 ID"
//	@Param		body	body		updateRelationshipRequest	true	"关系信息"
//	@Success	200		{object}	Envelope
//	@Failure	400		{object}	Envelope
//	@Failure	404		{object}	Envelope
//	@Router		/relationships/{id} [put]
func (s *Server) updateRelationship(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	var req updateRelationshipRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "请求参数不合法: "+err.Error(), nil)
		return
	}
	updated, err := s.characters.UpdateRelationship(c.Request.Context(), c.Param("id"), service.RelationshipInput{
		RelationType: domain.RelationType(req.RelationType),
		Strength:     req.Strength,
		Description:  req.Description,
	})
	if err != nil {
		s.failFromError(c, err)
		return
	}
	OK(c, toRelationshipResponse(*updated))
}

// deleteRelationship 删除人物关系。
//
//	@Summary	删除人物关系
//	@Tags		characters
//	@Produce	json
//	@Param		id	path		string	true	"关系 ID"
//	@Success	200	{object}	Envelope
//	@Failure	404	{object}	Envelope
//	@Router		/relationships/{id} [delete]
func (s *Server) deleteRelationship(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	if err := s.characters.DeleteRelationship(c.Request.Context(), c.Param("id")); err != nil {
		s.failFromError(c, err)
		return
	}
	OK(c, gin.H{"deleted": true})
}
