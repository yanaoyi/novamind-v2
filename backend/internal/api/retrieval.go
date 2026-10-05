package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type retrievalSearchRequest struct {
	WorkKind string `json:"work_kind" binding:"required,oneof=original creative"`
	WorkID   string `json:"work_id" binding:"required"`
	Query    string `json:"query" binding:"required"`
	TopK     int    `json:"top_k"`
}

type retrievalHit struct {
	ChunkID   string   `json:"chunk_id"`
	RefKind   string   `json:"ref_kind"`
	RefID     *string  `json:"ref_id"`
	Seq       int      `json:"seq"`
	Content   string   `json:"content"`
	Score     float64  `json:"score"`
	MatchFrom []string `json:"match_from"`
}

// retrievalSearch 是检索调试接口（Phase 9 §9.1.5）：让"AI 到底能翻到哪些旧账"可以直接看。
//
//	@Summary	检索调试
//	@Tags		retrieval
//	@Accept		json
//	@Produce	json
//	@Param		body	body		retrievalSearchRequest	true	"检索参数"
//	@Success	200		{object}	Envelope
//	@Router		/retrieval/search [post]
func (s *Server) retrievalSearch(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	var req retrievalSearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "请求参数不合法: "+err.Error(), nil)
		return
	}
	hits, err := s.retrieval.Search(c.Request.Context(), req.WorkKind, req.WorkID, req.Query, req.TopK)
	if err != nil {
		s.failFromError(c, err)
		return
	}
	items := make([]retrievalHit, 0, len(hits))
	for _, h := range hits {
		items = append(items, retrievalHit{
			ChunkID: h.ID, RefKind: h.RefKind, RefID: h.RefID, Seq: h.Seq,
			Content: h.Content, Score: h.Score, MatchFrom: h.MatchFrom,
		})
	}
	OK(c, gin.H{"items": items, "total": len(items), "query": req.Query})
}
