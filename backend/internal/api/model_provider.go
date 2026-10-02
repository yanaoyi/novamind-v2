package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/yanaoyi/novamindv2/backend/internal/domain"
	"github.com/yanaoyi/novamindv2/backend/internal/service"
)

// ---------- 请求 DTO ----------

type modelProviderRequest struct {
	Name        string  `json:"name" binding:"required" example:"DeepSeek"`
	Provider    string  `json:"provider" binding:"required,oneof=OPENAI_COMPATIBLE ANTHROPIC" example:"OPENAI_COMPATIBLE"`
	APIBase     string  `json:"api_base" binding:"required" example:"https://api.deepseek.com/v1"`
	APIKey      string  `json:"api_key" example:"sk-***"`
	ModelName   string  `json:"model_name" binding:"required" example:"deepseek-chat"`
	Purpose     string  `json:"purpose" binding:"omitempty,oneof=chat embedding both" example:"chat"`
	Temperature float64 `json:"temperature" example:"0.7"`
	MaxTokens   int     `json:"max_tokens" example:"4096"`
	TimeoutSec  int     `json:"timeout_sec" example:"120"`
	Enabled     bool    `json:"enabled" example:"true"`
	IsDefault   bool    `json:"is_default" example:"true"`
	Notes       string  `json:"notes"`
}

// ---------- 响应 DTO ----------

// ModelProviderResponse 是模型配置对外表示。
// 注意：**不包含任何形式的密钥**（明文、密文、尾号都不给），只给 has_api_key 布尔值。
type ModelProviderResponse struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Provider    string    `json:"provider"`
	APIBase     string    `json:"api_base"`
	ModelName   string    `json:"model_name"`
	Purpose     string    `json:"purpose"`
	Temperature float64   `json:"temperature"`
	MaxTokens   int       `json:"max_tokens"`
	TimeoutSec  int       `json:"timeout_sec"`
	Enabled     bool      `json:"enabled"`
	IsDefault   bool      `json:"is_default"`
	Notes       string    `json:"notes"`
	HasAPIKey   bool      `json:"has_api_key"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ModelProviderTestResponse 是连通性测试结果。
type ModelProviderTestResponse struct {
	OK           bool   `json:"ok"`
	Model        string `json:"model"`
	Reply        string `json:"reply"`
	LatencyMS    int64  `json:"latency_ms"`
	TotalTokens  int    `json:"total_tokens"`
	ErrorMessage string `json:"error_message,omitempty"`
}

// PromptMetaResponse 是 Prompt 模板清单项。
type PromptMetaResponse struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Path    string `json:"path"`
}

func toModelProviderResponse(p domain.ModelProvider) ModelProviderResponse {
	return ModelProviderResponse{
		ID: p.ID, Name: p.Name, Provider: string(p.Provider), APIBase: p.APIBase,
		ModelName: p.ModelName, Purpose: string(p.Purpose),
		Temperature: p.Temperature, MaxTokens: p.MaxTokens, TimeoutSec: p.TimeoutSec,
		Enabled: p.Enabled, IsDefault: p.IsDefault, Notes: p.Notes, HasAPIKey: p.HasAPIKey,
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

func (req modelProviderRequest) toInput() service.ProviderInput {
	return service.ProviderInput{
		Name: req.Name, Provider: domain.ProviderType(req.Provider), APIBase: req.APIBase,
		APIKey: req.APIKey, ModelName: req.ModelName, Purpose: domain.ProviderPurpose(req.Purpose),
		Temperature: req.Temperature, MaxTokens: req.MaxTokens, TimeoutSec: req.TimeoutSec,
		Enabled: req.Enabled, IsDefault: req.IsDefault, Notes: req.Notes,
	}
}

// ---------- Handlers ----------

// listModelProviders 模型配置列表。
//
//	@Summary	模型配置列表
//	@Tags		ai
//	@Produce	json
//	@Success	200	{object}	Envelope
//	@Router		/model-providers [get]
func (s *Server) listModelProviders(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	items, err := s.providers.List(c.Request.Context())
	if err != nil {
		s.failFromError(c, err)
		return
	}
	resp := make([]ModelProviderResponse, 0, len(items))
	for _, it := range items {
		resp = append(resp, toModelProviderResponse(it))
	}
	OK(c, gin.H{"items": resp, "total": len(resp)})
}

// createModelProvider 新增模型配置。
//
//	@Summary		新增模型配置
//	@Description	api_key 会用 NOVAMIND_SECRET 加密后入库；接口永不返回密钥。
//	@Tags			ai
//	@Accept			json
//	@Produce		json
//	@Param			body	body		modelProviderRequest	true	"模型配置"
//	@Success		201		{object}	Envelope
//	@Failure		400		{object}	Envelope
//	@Failure		409		{object}	Envelope
//	@Router			/model-providers [post]
func (s *Server) createModelProvider(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	var req modelProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "请求参数不合法: "+err.Error(), nil)
		return
	}
	created, err := s.providers.Create(c.Request.Context(), req.toInput())
	if err != nil {
		s.failFromError(c, err)
		return
	}
	Created(c, toModelProviderResponse(*created))
}

// getModelProvider 模型配置详情。
//
//	@Summary	模型配置详情
//	@Tags		ai
//	@Produce	json
//	@Param		id	path		string	true	"配置 ID"
//	@Success	200	{object}	Envelope
//	@Failure	404	{object}	Envelope
//	@Router		/model-providers/{id} [get]
func (s *Server) getModelProvider(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	p, err := s.providers.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		s.failFromError(c, err)
		return
	}
	OK(c, toModelProviderResponse(*p))
}

// updateModelProvider 更新模型配置。
//
//	@Summary		更新模型配置
//	@Description	api_key 留空表示保持原密钥不变。
//	@Tags			ai
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string					true	"配置 ID"
//	@Param			body	body		modelProviderRequest	true	"模型配置"
//	@Success		200		{object}	Envelope
//	@Failure		400		{object}	Envelope
//	@Failure		404		{object}	Envelope
//	@Router			/model-providers/{id} [put]
func (s *Server) updateModelProvider(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	var req modelProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "请求参数不合法: "+err.Error(), nil)
		return
	}
	updated, err := s.providers.Update(c.Request.Context(), c.Param("id"), req.toInput())
	if err != nil {
		s.failFromError(c, err)
		return
	}
	OK(c, toModelProviderResponse(*updated))
}

// deleteModelProvider 删除模型配置。
//
//	@Summary	删除模型配置
//	@Tags		ai
//	@Produce	json
//	@Param		id	path		string	true	"配置 ID"
//	@Success	200	{object}	Envelope
//	@Failure	404	{object}	Envelope
//	@Router		/model-providers/{id} [delete]
func (s *Server) deleteModelProvider(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	if err := s.providers.Delete(c.Request.Context(), c.Param("id")); err != nil {
		s.failFromError(c, err)
		return
	}
	OK(c, gin.H{"deleted": true})
}

// setDefaultModelProvider 设为默认模型。
//
//	@Summary	设为默认模型
//	@Tags		ai
//	@Produce	json
//	@Param		id	path		string	true	"配置 ID"
//	@Success	200	{object}	Envelope
//	@Failure	404	{object}	Envelope
//	@Router		/model-providers/{id}/default [post]
func (s *Server) setDefaultModelProvider(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	p, err := s.providers.SetDefault(c.Request.Context(), c.Param("id"))
	if err != nil {
		s.failFromError(c, err)
		return
	}
	OK(c, toModelProviderResponse(*p))
}

// testModelProvider 连通性测试。
//
//	@Summary		测试模型配置
//	@Description	用一条最小请求真实调用上游，验证地址/密钥/模型名是否可用（会消耗极少量 token）。
//	@Tags			ai
//	@Produce		json
//	@Param			id	path		string	true	"配置 ID"
//	@Success		200	{object}	Envelope
//	@Failure		400	{object}	Envelope
//	@Failure		404	{object}	Envelope
//	@Router			/model-providers/{id}/test [post]
func (s *Server) testModelProvider(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	result, err := s.providers.Test(c.Request.Context(), c.Param("id"))
	if err != nil {
		s.failFromError(c, err)
		return
	}
	OK(c, ModelProviderTestResponse{
		OK: result.OK, Model: result.Model, Reply: result.Reply,
		LatencyMS: result.LatencyMS, TotalTokens: result.TotalTokens, ErrorMessage: result.ErrorMessage,
	})
}

// listPrompts Prompt 模板清单（版本化，SPEC.md §17）。
//
//	@Summary	Prompt 模板清单
//	@Tags		ai
//	@Produce	json
//	@Success	200	{object}	Envelope
//	@Router		/prompts [get]
func (s *Server) listPrompts(c *gin.Context) {
	if s.prompts == nil {
		OK(c, gin.H{"items": []PromptMetaResponse{}, "total": 0})
		return
	}
	metas := s.prompts.List()
	resp := make([]PromptMetaResponse, 0, len(metas))
	for _, m := range metas {
		resp = append(resp, PromptMetaResponse{Name: m.Name, Version: m.Version, Path: m.Path})
	}
	OK(c, gin.H{"items": resp, "total": len(resp)})
}
