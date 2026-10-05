package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/yanaoyi/novamindv2/backend/internal/domain"
	"github.com/yanaoyi/novamindv2/backend/internal/service"
)

// ---------- 请求 DTO ----------

type createCreativeRequest struct {
	ProjectID   string `json:"project_id" binding:"required" example:"CREATIVE 类型文章的 ID"`
	Title       string `json:"title" binding:"required" example:"暗涌·另一个结局"`
	Description string `json:"description"`
}

type updateCreativeRequest struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Status      *string `json:"status" binding:"omitempty,oneof=DRAFT WRITING FINISHED"`
}

// inheritanceWeights 是各维度继承权重（0-100）。
type inheritanceWeights struct {
	Personality  int `json:"personality" example:"90"`
	Values       int `json:"values" example:"80"`
	Motivation   int `json:"motivation" example:"70"`
	Behavior     int `json:"behavior" example:"60"`
	SpeechStyle  int `json:"speech_style" example:"30"`
	Background   int `json:"background" example:"10"`
	Ability      int `json:"ability" example:"0"`
	Relationship int `json:"relationship_pattern" example:"50"`
}

type inheritCharacterRequest struct {
	SourceCharacterID string             `json:"source_character_id" binding:"required"`
	Name              string             `json:"name"`
	Description       string             `json:"description"`
	Importance        int                `json:"importance" example:"5"`
	Weights           inheritanceWeights `json:"weights"`
}

type newCreativeCharacterRequest struct {
	Name        string              `json:"name" binding:"required"`
	Description string              `json:"description"`
	Importance  int                 `json:"importance"`
	DNA         domain.CharacterDNA `json:"dna"`
}

type fuseSourcesRequest struct {
	CharacterID string `json:"character_id" binding:"required"`
	Weight      int    `json:"weight" example:"60"`
}

type fuseCharacterRequest struct {
	Name        string               `json:"name" binding:"required" example:"林述"`
	Description string               `json:"description"`
	Importance  int                  `json:"importance"`
	Sources     []fuseSourcesRequest `json:"sources" binding:"required,min=2"`
}

type updateCreativeCharacterRequest struct {
	Name        *string              `json:"name"`
	Description *string              `json:"description"`
	DNA         *domain.CharacterDNA `json:"dna"`
	Importance  *int                 `json:"importance"`
	IsLocked    *bool                `json:"is_locked"`
}

// ---------- 响应 DTO ----------

// CreativeWorkResponse 是二创作品对外表示。
type CreativeWorkResponse struct {
	ID                string    `json:"id"`
	ProjectID         string    `json:"project_id"`
	OriginalWorkID    string    `json:"original_work_id"`
	Title             string    `json:"title"`
	Description       string    `json:"description"`
	Status            string    `json:"status"`
	DivergencePointID *string   `json:"divergence_point_id"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// InheritanceRuleResponse 是继承权重对外表示。
type InheritanceRuleResponse struct {
	ID                string `json:"id"`
	SourceCharacterID string `json:"source_character_id"`
	Personality       int    `json:"personality"`
	Values            int    `json:"values"`
	Motivation        int    `json:"motivation"`
	Behavior          int    `json:"behavior"`
	SpeechStyle       int    `json:"speech_style"`
	Background        int    `json:"background"`
	Ability           int    `json:"ability"`
	Relationship      int    `json:"relationship_pattern"`
}

// CreativeCharacterResponse 是二创人物对外表示。
type CreativeCharacterResponse struct {
	ID                string                     `json:"id"`
	CreativeWorkID    string                     `json:"creative_work_id"`
	Name              string                     `json:"name"`
	Description       string                     `json:"description"`
	SourceType        string                     `json:"source_type"`
	SourceCharacterID *string                    `json:"source_character_id"`
	DNA               domain.CharacterDNA        `json:"dna"`
	FusionSources     []domain.FusionSource      `json:"fusion_sources"`
	FusionDetail      []domain.FusionAttribution `json:"fusion_detail"`
	Importance        int                        `json:"importance"`
	IsLocked          bool                       `json:"is_locked"`
	Rules             []InheritanceRuleResponse  `json:"rules,omitempty"`
	CreatedAt         time.Time                  `json:"created_at"`
	UpdatedAt         time.Time                  `json:"updated_at"`
}

// MappingResponse 是映射对外表示。
type MappingResponse struct {
	ID             string    `json:"id"`
	CreativeWorkID string    `json:"creative_work_id"`
	OriginalType   string    `json:"original_type"`
	OriginalID     string    `json:"original_id"`
	CreativeType   string    `json:"creative_type"`
	CreativeID     string    `json:"creative_id"`
	MappingType    string    `json:"mapping_type"`
	Description    string    `json:"description"`
	CreatedAt      time.Time `json:"created_at"`
}

func toCreativeWorkResponse(w domain.CreativeWork) CreativeWorkResponse {
	return CreativeWorkResponse{
		ID: w.ID, ProjectID: w.ProjectID, OriginalWorkID: w.OriginalWorkID,
		Title: w.Title, Description: w.Description, Status: string(w.Status),
		DivergencePointID: w.DivergencePointID,
		CreatedAt:         w.CreatedAt, UpdatedAt: w.UpdatedAt,
	}
}

func toInheritanceRuleResponse(r domain.InheritanceRule) InheritanceRuleResponse {
	return InheritanceRuleResponse{
		ID: r.ID, SourceCharacterID: r.SourceCharacterID,
		Personality: r.PersonalityWeight, Values: r.ValueWeight, Motivation: r.MotivationWeight,
		Behavior: r.BehaviorWeight, SpeechStyle: r.SpeechWeight, Background: r.BackgroundWeight,
		Ability: r.AbilityWeight, Relationship: r.RelationshipWeight,
	}
}

func toCreativeCharacterResponse(c domain.CreativeCharacter, rules []domain.InheritanceRule) CreativeCharacterResponse {
	resp := CreativeCharacterResponse{
		ID: c.ID, CreativeWorkID: c.CreativeWorkID, Name: c.Name, Description: c.Description,
		SourceType: string(c.SourceType), SourceCharacterID: c.SourceCharacterID, DNA: c.DNA,
		FusionSources: c.FusionSources, FusionDetail: c.FusionDetail,
		Importance: c.Importance, IsLocked: c.IsLocked,
		CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
	}
	if resp.FusionSources == nil {
		resp.FusionSources = []domain.FusionSource{}
	}
	if resp.FusionDetail == nil {
		resp.FusionDetail = []domain.FusionAttribution{}
	}
	for _, r := range rules {
		resp.Rules = append(resp.Rules, toInheritanceRuleResponse(r))
	}
	return resp
}

func toMappingResponse(m domain.OriginalCreativeMapping) MappingResponse {
	return MappingResponse{
		ID: m.ID, CreativeWorkID: m.CreativeWorkID,
		OriginalType: m.OriginalType, OriginalID: m.OriginalID,
		CreativeType: m.CreativeType, CreativeID: m.CreativeID,
		MappingType: string(m.MappingType), Description: m.Description, CreatedAt: m.CreatedAt,
	}
}

// ---------- Handlers ----------

// createCreativeWork 从原著创建二创作品。
//
//	@Summary		创建二创作品
//	@Description	给一个 CREATIVE 类型文章绑定一部原著，并建立二创作品。
//	@Tags			creative
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string					true	"原著 ID"
//	@Param			body	body		createCreativeRequest	true	"二创作品信息"
//	@Success		201		{object}	Envelope
//	@Failure		400		{object}	Envelope
//	@Failure		404		{object}	Envelope
//	@Failure		409		{object}	Envelope
//	@Router			/original/{id}/create-creative [post]
func (s *Server) createCreativeWork(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	var req createCreativeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "请求参数不合法: "+err.Error(), nil)
		return
	}
	work, err := s.creative.CreateWork(c.Request.Context(), service.CreateWorkInput{
		ProjectID: req.ProjectID, OriginalWorkID: c.Param("id"),
		Title: req.Title, Description: req.Description,
	})
	if err != nil {
		s.failFromError(c, err)
		return
	}
	Created(c, toCreativeWorkResponse(*work))
}

// getCreativeWork 二创作品详情。
//
//	@Summary	二创作品详情
//	@Tags		creative
//	@Produce	json
//	@Param		id	path		string	true	"二创作品 ID"
//	@Success	200	{object}	Envelope
//	@Failure	404	{object}	Envelope
//	@Router		/creative/{id} [get]
func (s *Server) getCreativeWork(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	work, err := s.creative.GetWork(c.Request.Context(), c.Param("id"))
	if err != nil {
		s.failFromError(c, err)
		return
	}
	OK(c, toCreativeWorkResponse(*work))
}

// updateCreativeWork 更新二创作品。
//
//	@Summary	更新二创作品
//	@Tags		creative
//	@Accept		json
//	@Produce	json
//	@Param		id		path		string					true	"二创作品 ID"
//	@Param		body	body		updateCreativeRequest	true	"待更新字段"
//	@Success	200		{object}	Envelope
//	@Failure	404		{object}	Envelope
//	@Router		/creative/{id} [put]
func (s *Server) updateCreativeWork(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	var req updateCreativeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "请求参数不合法: "+err.Error(), nil)
		return
	}
	in := service.UpdateWorkInput{Title: req.Title, Description: req.Description}
	if req.Status != nil {
		status := domain.CreativeWorkStatus(*req.Status)
		in.Status = &status
	}
	work, err := s.creative.UpdateWork(c.Request.Context(), c.Param("id"), in)
	if err != nil {
		s.failFromError(c, err)
		return
	}
	OK(c, toCreativeWorkResponse(*work))
}

// listCreativeWorks 列出某原著的二创作品。
//
//	@Summary	原著下的二创作品
//	@Tags		creative
//	@Produce	json
//	@Param		id	path		string	true	"原著 ID"
//	@Success	200	{object}	Envelope
//	@Router		/original/{id}/creative-works [get]
func (s *Server) listCreativeWorks(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	items, err := s.creative.ListWorksByOriginal(c.Request.Context(), c.Param("id"))
	if err != nil {
		s.failFromError(c, err)
		return
	}
	resp := make([]CreativeWorkResponse, 0, len(items))
	for _, w := range items {
		resp = append(resp, toCreativeWorkResponse(w))
	}
	OK(c, gin.H{"items": resp, "total": len(resp)})
}

// listCreativeCharacters 二创人物列表。
//
//	@Summary	二创人物列表
//	@Tags		creative
//	@Produce	json
//	@Param		id	path		string	true	"二创作品 ID"
//	@Success	200	{object}	Envelope
//	@Router		/creative/{id}/characters [get]
func (s *Server) listCreativeCharacters(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	items, err := s.creative.ListCharacters(c.Request.Context(), c.Param("id"))
	if err != nil {
		s.failFromError(c, err)
		return
	}
	resp := make([]CreativeCharacterResponse, 0, len(items))
	for _, item := range items {
		resp = append(resp, toCreativeCharacterResponse(item, nil))
	}
	OK(c, gin.H{"items": resp, "total": len(resp)})
}

// inheritCreativeCharacter 从原著人物继承。
//
//	@Summary		继承原著人物
//	@Description	按各维度权重从原著人物 DNA 派生二创人物 DNA（权重 0 表示不继承）。
//	@Tags			creative
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string						true	"二创作品 ID"
//	@Param			body	body		inheritCharacterRequest		true	"来源人物与继承权重"
//	@Success		201		{object}	Envelope
//	@Failure		400		{object}	Envelope
//	@Failure		404		{object}	Envelope
//	@Router			/creative/{id}/characters/inherit [post]
func (s *Server) inheritCreativeCharacter(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	var req inheritCharacterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "请求参数不合法: "+err.Error(), nil)
		return
	}
	rule := domain.InheritanceRule{
		PersonalityWeight: req.Weights.Personality, ValueWeight: req.Weights.Values,
		MotivationWeight: req.Weights.Motivation, BehaviorWeight: req.Weights.Behavior,
		SpeechWeight: req.Weights.SpeechStyle, BackgroundWeight: req.Weights.Background,
		AbilityWeight: req.Weights.Ability, RelationshipWeight: req.Weights.Relationship,
	}
	character, err := s.creative.InheritCharacter(c.Request.Context(), c.Param("id"), service.InheritInput{
		SourceCharacterID: req.SourceCharacterID,
		Name:              req.Name, Description: req.Description, Importance: req.Importance,
		Rule: rule,
	})
	if err != nil {
		s.failFromError(c, err)
		return
	}
	s.snapshotCharacter(c.Request.Context(), character.ID, "继承原著人物")
	Created(c, toCreativeCharacterResponse(*character, nil))
}

// createNewCreativeCharacter 创建原创人物。
//
//	@Summary	创建原创二创人物
//	@Tags		creative
//	@Accept		json
//	@Produce	json
//	@Param		id		path		string							true	"二创作品 ID"
//	@Param		body	body		newCreativeCharacterRequest		true	"人物信息"
//	@Success	201		{object}	Envelope
//	@Failure	400		{object}	Envelope
//	@Router			/creative/{id}/characters/new [post]
func (s *Server) createNewCreativeCharacter(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	var req newCreativeCharacterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "请求参数不合法: "+err.Error(), nil)
		return
	}
	character, err := s.creative.CreateNewCharacter(c.Request.Context(), c.Param("id"), service.CreateNewCharacterInput{
		Name: req.Name, Description: req.Description, DNA: req.DNA, Importance: req.Importance,
	})
	if err != nil {
		s.failFromError(c, err)
		return
	}
	s.snapshotCharacter(c.Request.Context(), character.ID, "新增二创人物")
	Created(c, toCreativeCharacterResponse(*character, nil))
}

// fuseCreativeCharacters 人物融合。
//
//	@Summary		人物融合
//	@Description	把多个二创人物融合成一个新人物，并生成每个 DNA 维度的来源说明（规格书 §20）。
//	@Tags			creative
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string					true	"二创作品 ID"
//	@Param			body	body		fuseCharacterRequest	true	"融合来源与整体权重"
//	@Success		201		{object}	Envelope
//	@Failure		400		{object}	Envelope
//	@Router			/creative/{id}/characters/fuse [post]
func (s *Server) fuseCreativeCharacters(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	var req fuseCharacterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "请求参数不合法: "+err.Error(), nil)
		return
	}
	sources := make([]service.FusionInputItem, 0, len(req.Sources))
	for _, src := range req.Sources {
		sources = append(sources, service.FusionInputItem{CharacterID: src.CharacterID, Weight: src.Weight})
	}
	character, err := s.creative.FuseCharacter(c.Request.Context(), c.Param("id"), service.FuseInput{
		Name: req.Name, Description: req.Description, Importance: req.Importance, Sources: sources,
	})
	if err != nil {
		s.failFromError(c, err)
		return
	}
	s.snapshotCharacter(c.Request.Context(), character.ID, "人物融合")
	Created(c, toCreativeCharacterResponse(*character, nil))
}

// getCreativeCharacter 二创人物详情（含继承权重）。
//
//	@Summary	二创人物详情
//	@Tags		creative
//	@Produce	json
//	@Param		id	path		string	true	"二创人物 ID"
//	@Success	200	{object}	Envelope
//	@Failure	404	{object}	Envelope
//	@Router		/creative-characters/{id} [get]
func (s *Server) getCreativeCharacter(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	detail, err := s.creative.GetCharacter(c.Request.Context(), c.Param("id"))
	if err != nil {
		s.failFromError(c, err)
		return
	}
	OK(c, toCreativeCharacterResponse(detail.Character, detail.Rules))
}

// updateCreativeCharacter 更新二创人物（作者手改 DNA/描述）。
//
//	@Summary	更新二创人物
//	@Tags		creative
//	@Accept		json
//	@Produce	json
//	@Param		id		path		string							true	"二创人物 ID"
//	@Param		body	body		updateCreativeCharacterRequest	true	"待更新字段"
//	@Success	200		{object}	Envelope
//	@Failure	404		{object}	Envelope
//	@Router		/creative-characters/{id} [put]
func (s *Server) updateCreativeCharacter(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	var req updateCreativeCharacterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "请求参数不合法: "+err.Error(), nil)
		return
	}
	character, err := s.creative.UpdateCharacter(c.Request.Context(), c.Param("id"), service.UpdateCharacterInput{
		Name: req.Name, Description: req.Description, DNA: req.DNA,
		Importance: req.Importance, IsLocked: req.IsLocked,
	})
	if err != nil {
		s.failFromError(c, err)
		return
	}
	s.snapshotCharacter(c.Request.Context(), character.ID, "作者修改人物")
	OK(c, toCreativeCharacterResponse(*character, nil))
}

// deleteCreativeCharacter 删除二创人物。
//
//	@Summary	删除二创人物
//	@Tags		creative
//	@Produce	json
//	@Param		id	path		string	true	"二创人物 ID"
//	@Success	200	{object}	Envelope
//	@Failure	404	{object}	Envelope
//	@Router		/creative-characters/{id} [delete]
func (s *Server) deleteCreativeCharacter(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	if err := s.creative.DeleteCharacter(c.Request.Context(), c.Param("id")); err != nil {
		s.failFromError(c, err)
		return
	}
	OK(c, gin.H{"deleted": true})
}

// listCreativeMappings 映射列表。
//
//	@Summary	原著↔二创映射列表
//	@Tags		creative
//	@Produce	json
//	@Param		id	path		string	true	"二创作品 ID"
//	@Success	200	{object}	Envelope
//	@Router		/creative/{id}/mappings [get]
func (s *Server) listCreativeMappings(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	items, err := s.creative.ListMappings(c.Request.Context(), c.Param("id"))
	if err != nil {
		s.failFromError(c, err)
		return
	}
	resp := make([]MappingResponse, 0, len(items))
	for _, m := range items {
		resp = append(resp, toMappingResponse(m))
	}
	OK(c, gin.H{"items": resp, "total": len(resp)})
}

// deleteCreativeMapping 删除映射。
//
//	@Summary	删除映射
//	@Tags		creative
//	@Produce	json
//	@Param		id	path		string	true	"映射 ID"
//	@Success	200	{object}	Envelope
//	@Failure	404	{object}	Envelope
//	@Router		/mappings/{id} [delete]
func (s *Server) deleteCreativeMapping(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	if err := s.creative.DeleteMapping(c.Request.Context(), c.Param("id")); err != nil {
		s.failFromError(c, err)
		return
	}
	OK(c, gin.H{"deleted": true})
}
