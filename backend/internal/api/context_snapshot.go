package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/yanaoyi/novamindv2/backend/internal/domain"
)

// ContextSnapshotResponse 是快照对外表示（列表不带 payload）。
type ContextSnapshotResponse struct {
	ID             string         `json:"id"`
	CreativeWorkID string         `json:"creative_work_id"`
	ChapterID      *string        `json:"chapter_id"`
	Kind           string         `json:"kind"`
	Snapshot       map[string]any `json:"snapshot,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
}

func toSnapshotResponse(s domain.ContextSnapshot, withPayload bool) ContextSnapshotResponse {
	out := ContextSnapshotResponse{
		ID: s.ID, CreativeWorkID: s.CreativeWorkID, ChapterID: s.ChapterID,
		Kind: string(s.Kind), CreatedAt: s.CreatedAt,
	}
	if withPayload {
		out.Snapshot = s.Snapshot
	}
	return out
}

// listChapterSnapshots 列出某章节的上下文快照（"当时到底给了 AI 什么"）。
//
//	@Summary	章节的上下文快照列表
//	@Tags		context
//	@Produce	json
//	@Param		id	path		string	true	"章节 ID"
//	@Success	200	{object}	Envelope
//	@Router		/chapters/{id}/snapshots [get]
func (s *Server) listChapterSnapshots(c *gin.Context) {
	if !s.requireServices(c) || !s.requireSnapshots(c) {
		return
	}
	items, err := s.snapshots.ListByChapter(c.Request.Context(), c.Param("id"))
	if err != nil {
		s.failFromError(c, err)
		return
	}
	resp := make([]ContextSnapshotResponse, 0, len(items))
	for _, item := range items {
		resp = append(resp, toSnapshotResponse(item, false))
	}
	OK(c, gin.H{"items": resp, "total": len(resp)})
}

// getContextSnapshot 取快照详情（含 8 段 + 检索来源 + 模型与 prompt 版本）。
//
//	@Summary	上下文快照详情
//	@Tags		context
//	@Produce	json
//	@Param		id	path		string	true	"快照 ID"
//	@Success	200	{object}	Envelope
//	@Failure	404	{object}	Envelope
//	@Router		/snapshots/{id} [get]
func (s *Server) getContextSnapshot(c *gin.Context) {
	if !s.requireServices(c) || !s.requireSnapshots(c) {
		return
	}
	item, err := s.snapshots.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		s.failFromError(c, err)
		return
	}
	OK(c, toSnapshotResponse(*item, true))
}

// requireSnapshots 确认快照服务已就绪（未就绪返回 503 而不是 panic 或假 404）。
func (s *Server) requireSnapshots(c *gin.Context) bool {
	if s.snapshots == nil {
		Fail(c, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "服务未就绪：快照服务未初始化", nil)
		return false
	}
	return true
}
