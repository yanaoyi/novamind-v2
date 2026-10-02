package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/yanaoyi/novamindv2/backend/internal/domain"
	"github.com/yanaoyi/novamindv2/backend/internal/repository"
)

type enqueueAnalysisRequest struct {
	Stage string `json:"stage" binding:"required,oneof=chapter_summary character_extract world_extract plot_extract" example:"character_extract"`
}

type approveProposalRequest struct {
	// Payload 是作者修改后的内容（可空：不改就直接通过）
	Payload map[string]any `json:"payload"`
	Note    string         `json:"note" example:"人物描述改成更贴近原文的写法"`
}

type rejectProposalRequest struct {
	Note string `json:"note" example:"原文没有依据"`
}

// ProposalResponse 是提案对外表示。
type ProposalResponse struct {
	ID         string         `json:"id"`
	WorkID     string         `json:"work_id"`
	TaskID     *string        `json:"task_id"`
	Stage      string         `json:"stage"`
	EntityType string         `json:"entity_type"`
	Title      string         `json:"title"`
	Payload    map[string]any `json:"payload"`
	Evidence   string         `json:"evidence"`
	Confidence int            `json:"confidence"`
	Status     string         `json:"status"`
	ReviewNote string         `json:"review_note"`
	ReviewedAt *time.Time     `json:"reviewed_at"`
	AppliedID  *string        `json:"applied_id"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
}

// ProposalListResponse 是提案列表响应。
type ProposalListResponse struct {
	Items    []ProposalResponse `json:"items"`
	Total    int64              `json:"total"`
	Page     int                `json:"page"`
	PageSize int                `json:"page_size"`
}

func toProposalResponse(p domain.AnalysisProposal) ProposalResponse {
	payload := p.Payload
	if payload == nil {
		payload = map[string]any{}
	}
	return ProposalResponse{
		ID: p.ID, WorkID: p.WorkID, TaskID: p.TaskID,
		Stage: string(p.Stage), EntityType: string(p.EntityType), Title: p.Title,
		Payload: payload, Evidence: p.Evidence, Confidence: p.Confidence,
		Status: string(p.Status), ReviewNote: p.ReviewNote,
		ReviewedAt: p.ReviewedAt, AppliedID: p.AppliedID,
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

// enqueueAnalysis 触发一个分析阶段（异步，返回 task_id）。
//
//	@Summary		触发原著分析
//	@Description	按阶段分析原著。AI 产出**只会进入待审核提案**，不会直接改写原著数据。
//	@Tags			analysis
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string					true	"原著 ID"
//	@Param			body	body		enqueueAnalysisRequest	true	"阶段"
//	@Success		202		{object}	Envelope
//	@Failure		400		{object}	Envelope
//	@Failure		404		{object}	Envelope
//	@Router			/original/{id}/analysis [post]
func (s *Server) enqueueAnalysis(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	var req enqueueAnalysisRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "请求参数不合法: "+err.Error(), nil)
		return
	}
	t, err := s.analysis.EnqueueStage(c.Request.Context(), c.Param("id"), domain.AnalysisStage(req.Stage))
	if err != nil {
		s.failFromError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, Envelope{Data: toTaskResponse(domainTask(*t)), TraceID: traceID(c)})
}

// listProposals 提案列表。
//
//	@Summary	分析提案列表
//	@Tags		analysis
//	@Produce	json
//	@Param		id			path		string	true	"原著 ID"
//	@Param		status		query		string	false	"状态"	Enums(PENDING, APPROVED, REJECTED)
//	@Param		stage		query		string	false	"阶段"
//	@Param		entity_type	query		string	false	"实体类型"
//	@Param		page		query		int		false	"页码"
//	@Param		page_size	query		int		false	"每页条数"
//	@Success	200			{object}	Envelope
//	@Router		/original/{id}/proposals [get]
func (s *Server) listProposals(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	page := parseInt(c.Query("page"), 1)
	pageSize := parseInt(c.Query("page_size"), 50)
	items, total, err := s.analysis.ListProposals(c.Request.Context(), repository.ProposalFilter{
		WorkID:     c.Param("id"),
		Status:     c.Query("status"),
		Stage:      c.Query("stage"),
		EntityType: c.Query("entity_type"),
		Page:       page,
		PageSize:   pageSize,
	})
	if err != nil {
		s.failFromError(c, err)
		return
	}
	resp := ProposalListResponse{
		Items: make([]ProposalResponse, 0, len(items)), Total: total, Page: page, PageSize: pageSize,
	}
	for _, p := range items {
		resp.Items = append(resp.Items, toProposalResponse(p))
	}
	OK(c, resp)
}

// getProposal 提案详情。
//
//	@Summary	提案详情
//	@Tags		analysis
//	@Produce	json
//	@Param		id	path		string	true	"提案 ID"
//	@Success	200	{object}	Envelope
//	@Failure	404	{object}	Envelope
//	@Router		/proposals/{id} [get]
func (s *Server) getProposal(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	p, err := s.analysis.GetProposal(c.Request.Context(), c.Param("id"))
	if err != nil {
		s.failFromError(c, err)
		return
	}
	OK(c, toProposalResponse(*p))
}

// approveProposal 审核通过（可带作者修改）。
//
//	@Summary		通过提案
//	@Description	作者确认后写入原著正式表；payload 可传修改后的内容（SPEC.md §22.3：AI 结果允许作者修改）。
//	@Tags			analysis
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string					true	"提案 ID"
//	@Param			body	body		approveProposalRequest	false	"修改内容与备注"
//	@Success		200		{object}	Envelope
//	@Failure		400		{object}	Envelope
//	@Failure		404		{object}	Envelope
//	@Failure		409		{object}	Envelope
//	@Router			/proposals/{id}/approve [post]
func (s *Server) approveProposal(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	var req approveProposalRequest
	if err := c.ShouldBindJSON(&req); err != nil && err.Error() != "EOF" {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "请求参数不合法: "+err.Error(), nil)
		return
	}
	p, err := s.analysis.ApproveProposal(c.Request.Context(), c.Param("id"), req.Payload, req.Note)
	if err != nil {
		s.failFromError(c, err)
		return
	}
	OK(c, toProposalResponse(*p))
}

// rejectProposal 驳回提案。
//
//	@Summary	驳回提案
//	@Tags		analysis
//	@Accept		json
//	@Produce	json
//	@Param		id		path		string					true	"提案 ID"
//	@Param		body	body		rejectProposalRequest	false	"备注"
//	@Success	200		{object}	Envelope
//	@Failure	404		{object}	Envelope
//	@Failure	409		{object}	Envelope
//	@Router		/proposals/{id}/reject [post]
func (s *Server) rejectProposal(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	var req rejectProposalRequest
	if err := c.ShouldBindJSON(&req); err != nil && err.Error() != "EOF" {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "请求参数不合法: "+err.Error(), nil)
		return
	}
	p, err := s.analysis.RejectProposal(c.Request.Context(), c.Param("id"), req.Note)
	if err != nil {
		s.failFromError(c, err)
		return
	}
	OK(c, toProposalResponse(*p))
}

// analysisSummary 提案统计。
//
//	@Summary	分析提案统计
//	@Tags		analysis
//	@Produce	json
//	@Param		id	path		string	true	"原著 ID"
//	@Success	200	{object}	Envelope
//	@Router		/original/{id}/analysis/summary [get]
func (s *Server) analysisSummary(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	summary, err := s.analysis.Summary(c.Request.Context(), c.Param("id"))
	if err != nil {
		s.failFromError(c, err)
		return
	}
	OK(c, gin.H{
		"pending": summary.Pending, "approved": summary.Approved, "rejected": summary.Rejected,
	})
}
