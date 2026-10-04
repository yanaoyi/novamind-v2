package api

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/yanaoyi/novamindv2/backend/internal/domain"
	"github.com/yanaoyi/novamindv2/backend/internal/service"
)

// ---------- 请求 ----------

type createOutlineRequest struct {
	Title   string                    `json:"title" binding:"required"`
	Summary string                    `json:"summary"`
	Version int                       `json:"version"`
	Source  string                    `json:"source"`
	Nodes   []domain.OutlineNodeInput `json:"nodes"`
}

type updateOutlineRequest struct {
	Title   *string `json:"title"`
	Summary *string `json:"summary"`
	Version *int    `json:"version"`
}

type replaceOutlineTreeRequest struct {
	Nodes []domain.OutlineNodeInput `json:"nodes" binding:"required"`
}

type createOutlineNodeRequest struct {
	ParentID   *string  `json:"parent_id"`
	Title      string   `json:"title" binding:"required"`
	Summary    string   `json:"summary"`
	Purpose    string   `json:"purpose"`
	Characters []string `json:"characters"`
	Location   string   `json:"location"`
	Conflict   string   `json:"conflict"`
	Outcome    string   `json:"outcome"`
}

type updateOutlineNodeRequest struct {
	Title      *string   `json:"title"`
	Summary    *string   `json:"summary"`
	Purpose    *string   `json:"purpose"`
	Characters *[]string `json:"characters"`
	Location   *string   `json:"location"`
	Conflict   *string   `json:"conflict"`
	Outcome    *string   `json:"outcome"`
	Sequence   *int      `json:"sequence"`
}

type snapshotOutlineRequest struct {
	Note string `json:"note"`
}

// ---------- 响应 ----------

type OutlineResponse struct {
	ID             string    `json:"id"`
	CreativeWorkID string    `json:"creative_work_id"`
	Title          string    `json:"title"`
	Summary        string    `json:"summary"`
	Version        int       `json:"version"`
	Source         string    `json:"source"`
	NodeCount      int       `json:"node_count"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func toOutlineResponse(o domain.Outline) OutlineResponse {
	return OutlineResponse{
		ID: o.ID, CreativeWorkID: o.CreativeWorkID, Title: o.Title, Summary: o.Summary,
		Version: o.Version, Source: string(o.Source), NodeCount: o.NodeCount,
		CreatedAt: o.CreatedAt, UpdatedAt: o.UpdatedAt,
	}
}

type OutlineNodeResponse struct {
	ID         string                `json:"id"`
	OutlineID  string                `json:"outline_id"`
	ParentID   *string               `json:"parent_id"`
	Level      int                   `json:"level"`
	LevelName  string                `json:"level_name"`
	Sequence   int                   `json:"sequence"`
	Title      string                `json:"title"`
	Summary    string                `json:"summary"`
	Purpose    string                `json:"purpose"`
	Characters []string              `json:"characters"`
	Location   string                `json:"location"`
	Conflict   string                `json:"conflict"`
	Outcome    string                `json:"outcome"`
	Children   []OutlineNodeResponse `json:"children"`
	CreatedAt  time.Time             `json:"created_at"`
	UpdatedAt  time.Time             `json:"updated_at"`
}

func toOutlineNodeResponse(n *domain.OutlineNodeTree) OutlineNodeResponse {
	out := OutlineNodeResponse{
		ID: n.ID, OutlineID: n.OutlineID, ParentID: n.ParentID, Level: int(n.Level),
		LevelName: n.Level.Name(), Sequence: n.Sequence, Title: n.Title, Summary: n.Summary,
		Purpose: n.Purpose, Characters: n.Characters, Location: n.Location,
		Conflict: n.Conflict, Outcome: n.Outcome,
		Children:  []OutlineNodeResponse{},
		CreatedAt: n.CreatedAt, UpdatedAt: n.UpdatedAt,
	}
	for _, child := range n.Children {
		out.Children = append(out.Children, toOutlineNodeResponse(child))
	}
	return out
}

func outlineDetailResponse(detail *service.OutlineDetail) gin.H {
	nodes := make([]OutlineNodeResponse, 0, len(detail.Nodes))
	for _, n := range detail.Nodes {
		nodes = append(nodes, toOutlineNodeResponse(n))
	}
	return gin.H{"outline": toOutlineResponse(detail.Outline), "nodes": nodes}
}

// ---------- 处理函数 ----------

// createOutline 新建大纲（可同时写入整棵树；AI 候选经作者确认后也走这里）。
//
//	@Summary		新建大纲
//	@Description	nodes 为空时创建空大纲；非空时按「卷 → 节 → 章」嵌套一次性写入。
//	@Tags			outline
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string					true	"二创作品 ID"
//	@Param			body	body		createOutlineRequest	true	"大纲"
//	@Success		201		{object}	Envelope
//	@Failure		400		{object}	Envelope
//	@Router			/creative/{id}/outlines [post]
func (s *Server) createOutline(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	var req createOutlineRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "请求参数不合法: "+err.Error(), nil)
		return
	}
	detail, err := s.outlines.CreateOutline(c.Request.Context(), c.Param("id"), service.CreateOutlineInput{
		Title: req.Title, Summary: req.Summary, Version: req.Version,
		Source: domain.OutlineSource(req.Source), Nodes: req.Nodes,
	})
	if err != nil {
		s.failFromError(c, err)
		return
	}
	s.snapshotOutlineTree(c.Request.Context(), detail.Outline.ID, "新建大纲")
	Created(c, outlineDetailResponse(detail))
}

// listOutlines 大纲列表。
//
//	@Summary	大纲列表
//	@Tags		outline
//	@Produce	json
//	@Param		id	path		string	true	"二创作品 ID"
//	@Success	200	{object}	Envelope
//	@Router		/creative/{id}/outlines [get]
func (s *Server) listOutlines(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	items, err := s.outlines.ListOutlines(c.Request.Context(), c.Param("id"))
	if err != nil {
		s.failFromError(c, err)
		return
	}
	resp := make([]OutlineResponse, 0, len(items))
	for _, o := range items {
		resp = append(resp, toOutlineResponse(o))
	}
	OK(c, gin.H{"items": resp, "total": len(resp)})
}

// getOutline 大纲详情（含节点树）。
//
//	@Summary	大纲详情
//	@Tags		outline
//	@Produce	json
//	@Param		id	path		string	true	"大纲 ID"
//	@Success	200	{object}	Envelope
//	@Failure	404	{object}	Envelope
//	@Router		/outlines/{id} [get]
func (s *Server) getOutline(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	detail, err := s.outlines.GetOutline(c.Request.Context(), c.Param("id"))
	if err != nil {
		s.failFromError(c, err)
		return
	}
	OK(c, outlineDetailResponse(detail))
}

// updateOutline 修改大纲元信息。
//
//	@Summary	修改大纲
//	@Tags		outline
//	@Accept		json
//	@Produce	json
//	@Param		id		path		string					true	"大纲 ID"
//	@Param		body	body		updateOutlineRequest	true	"标题 / 概要 / 版本号"
//	@Success	200		{object}	Envelope
//	@Router		/outlines/{id} [put]
func (s *Server) updateOutline(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	var req updateOutlineRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "请求参数不合法: "+err.Error(), nil)
		return
	}
	detail, err := s.outlines.UpdateOutline(c.Request.Context(), c.Param("id"), service.UpdateOutlineInput{
		Title: req.Title, Summary: req.Summary, Version: req.Version,
	})
	if err != nil {
		s.failFromError(c, err)
		return
	}
	s.snapshotOutlineTree(c.Request.Context(), c.Param("id"), "修改大纲信息")
	OK(c, outlineDetailResponse(detail))
}

// deleteOutline 删除大纲（连同节点）。
//
//	@Summary	删除大纲
//	@Tags		outline
//	@Produce	json
//	@Param		id	path		string	true	"大纲 ID"
//	@Success	200	{object}	Envelope
//	@Router		/outlines/{id} [delete]
func (s *Server) deleteOutline(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	// 删除前留一版，作者删错了还能从版本里看到曾经的结构
	s.snapshotOutlineTree(c.Request.Context(), c.Param("id"), "删除前自动备份")
	if err := s.outlines.DeleteOutline(c.Request.Context(), c.Param("id")); err != nil {
		s.failFromError(c, err)
		return
	}
	OK(c, gin.H{"deleted": true})
}

// replaceOutlineTree 用整棵树替换大纲内容。
//
//	@Summary		替换大纲树
//	@Description	作者在 AI 候选上改完后整体提交走这里；旧节点会被替换（历史版本仍保留）。
//	@Tags			outline
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string						true	"大纲 ID"
//	@Param			body	body		replaceOutlineTreeRequest	true	"节点树"
//	@Success		200		{object}	Envelope
//	@Router			/outlines/{id}/tree [put]
func (s *Server) replaceOutlineTree(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	var req replaceOutlineTreeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "请求参数不合法: "+err.Error(), nil)
		return
	}
	s.snapshotOutlineTree(c.Request.Context(), c.Param("id"), "替换前自动备份")
	detail, err := s.outlines.ReplaceTree(c.Request.Context(), c.Param("id"), req.Nodes)
	if err != nil {
		s.failFromError(c, err)
		return
	}
	s.snapshotOutlineTree(c.Request.Context(), c.Param("id"), "替换大纲树")
	OK(c, outlineDetailResponse(detail))
}

// createOutlineNode 在指定父节点下新增节点。
//
//	@Summary		新增大纲节点
//	@Description	层级由父节点推导：不传 parent_id 建「卷」，卷下建「节」，节下建「章」。
//	@Tags			outline
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string						true	"大纲 ID"
//	@Param			body	body		createOutlineNodeRequest	true	"节点"
//	@Success		201		{object}	Envelope
//	@Router			/outlines/{id}/nodes [post]
func (s *Server) createOutlineNode(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	var req createOutlineNodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "请求参数不合法: "+err.Error(), nil)
		return
	}
	node, err := s.outlines.CreateNode(c.Request.Context(), c.Param("id"), service.CreateNodeInput{
		ParentID: req.ParentID, Title: req.Title, Summary: req.Summary, Purpose: req.Purpose,
		Characters: req.Characters, Location: req.Location, Conflict: req.Conflict, Outcome: req.Outcome,
	})
	if err != nil {
		s.failFromError(c, err)
		return
	}
	s.snapshotOutlineTree(c.Request.Context(), c.Param("id"), "新增节点")
	Created(c, toOutlineNodeResponse(&domain.OutlineNodeTree{OutlineNode: *node, Children: []*domain.OutlineNodeTree{}}))
}

// updateOutlineNode 修改节点。
//
//	@Summary	修改大纲节点
//	@Tags		outline
//	@Accept		json
//	@Produce	json
//	@Param		id		path		string						true	"节点 ID"
//	@Param		body	body		updateOutlineNodeRequest	true	"节点字段"
//	@Success	200		{object}	Envelope
//	@Router		/outline-nodes/{id} [put]
func (s *Server) updateOutlineNode(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	var req updateOutlineNodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "请求参数不合法: "+err.Error(), nil)
		return
	}
	node, err := s.outlines.UpdateNode(c.Request.Context(), c.Param("id"), service.UpdateNodeInput{
		Title: req.Title, Summary: req.Summary, Purpose: req.Purpose, Characters: req.Characters,
		Location: req.Location, Conflict: req.Conflict, Outcome: req.Outcome, Sequence: req.Sequence,
	})
	if err != nil {
		s.failFromError(c, err)
		return
	}
	s.snapshotOutlineTree(c.Request.Context(), node.OutlineID, "修改节点")
	OK(c, toOutlineNodeResponse(&domain.OutlineNodeTree{OutlineNode: *node, Children: []*domain.OutlineNodeTree{}}))
}

// deleteOutlineNode 删除节点及其子树。
//
//	@Summary	删除大纲节点
//	@Tags		outline
//	@Produce	json
//	@Param		id	path		string	true	"节点 ID"
//	@Success	200	{object}	Envelope
//	@Router		/outline-nodes/{id} [delete]
func (s *Server) deleteOutlineNode(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	node, err := s.outlines.GetNode(c.Request.Context(), c.Param("id"))
	if err != nil {
		s.failFromError(c, err)
		return
	}
	s.snapshotOutlineTree(c.Request.Context(), node.OutlineID, "删除前自动备份")
	count, err := s.outlines.DeleteNode(c.Request.Context(), c.Param("id"))
	if err != nil {
		s.failFromError(c, err)
		return
	}
	OK(c, gin.H{"deleted": true, "deleted_nodes": count})
}

// materializeOutline 把大纲里的「章」落成写作系统的卷与章节。
//
//	@Summary		大纲落成章节
//	@Description	卷按标题复用，章节一律追加（不覆盖已写正文）；节标题会作为章节摘要前缀保留。
//	@Tags			outline
//	@Produce		json
//	@Param			id	path		string	true	"大纲 ID"
//	@Success		200	{object}	Envelope
//	@Router			/outlines/{id}/materialize [post]
func (s *Server) materializeOutline(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	result, err := s.outlines.Materialize(c.Request.Context(), c.Param("id"))
	if err != nil {
		s.failFromError(c, err)
		return
	}
	OK(c, result)
}

// snapshotOutlineTree 手动存一版大纲树快照。
//
//	@Summary	存一版大纲快照
//	@Tags		outline
//	@Accept		json
//	@Produce	json
//	@Param		id		path		string					true	"大纲 ID"
//	@Param		body	body		snapshotOutlineRequest	true	"备注"
//	@Success	201		{object}	Envelope
//	@Router		/outlines/{id}/versions [post]
func (s *Server) snapshotOutlineTreeHandler(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	var req snapshotOutlineRequest
	if err := bindOptionalJSON(c, &req); err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, err.Error(), nil)
		return
	}
	version, err := s.outlines.SnapshotTree(c.Request.Context(), c.Param("id"), req.Note)
	if err != nil {
		s.failFromError(c, err)
		return
	}
	if version == nil {
		OK(c, gin.H{"created": false, "reason": "内容与上一版相同，未新建版本"})
		return
	}
	Created(c, toVersionResponse(*version, false))
}

// listOutlineTreeVersions 大纲树版本列表。
//
//	@Summary	大纲版本列表
//	@Tags		outline
//	@Produce	json
//	@Param		id	path		string	true	"大纲 ID"
//	@Success	200	{object}	Envelope
//	@Router		/outlines/{id}/versions [get]
func (s *Server) listOutlineTreeVersions(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	items, err := s.outlines.ListVersions(c.Request.Context(), c.Param("id"))
	if err != nil {
		s.failFromError(c, err)
		return
	}
	resp := make([]EntityVersionResponse, 0, len(items))
	for _, v := range items {
		resp = append(resp, toVersionResponse(v, false))
	}
	OK(c, gin.H{"items": resp, "total": len(resp)})
}

// getOutlineTreeVersion 大纲版本详情。
//
//	@Summary	大纲版本详情
//	@Tags		outline
//	@Produce	json
//	@Param		id	path		string	true	"大纲 ID"
//	@Param		no	path		int		true	"版本号"
//	@Success	200	{object}	Envelope
//	@Router		/outlines/{id}/versions/{no} [get]
func (s *Server) getOutlineTreeVersion(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	no, err := parseIntParam(c, "no")
	if err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "版本号不合法", nil)
		return
	}
	version, err := s.outlines.GetVersion(c.Request.Context(), c.Param("id"), no)
	if err != nil {
		s.failFromError(c, err)
		return
	}
	OK(c, toVersionResponse(*version, true))
}

// restoreOutlineTreeVersion 恢复大纲到某一版。
//
//	@Summary		恢复大纲版本
//	@Description	整棵树替换为快照内容；已经从该大纲落成的章节不受影响。
//	@Tags			outline
//	@Produce		json
//	@Param			id	path		string	true	"大纲 ID"
//	@Param			no	path		int		true	"版本号"
//	@Success		200	{object}	Envelope
//	@Router			/outlines/{id}/versions/{no}/restore [post]
func (s *Server) restoreOutlineTreeVersion(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	no, err := parseIntParam(c, "no")
	if err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "版本号不合法", nil)
		return
	}
	detail, err := s.outlines.RestoreVersion(c.Request.Context(), c.Param("id"), no)
	if err != nil {
		s.failFromError(c, err)
		return
	}
	OK(c, outlineDetailResponse(detail))
}

// snapshotOutlineTree 是「尽力而为」的快照：失败只记日志，不影响作者本次操作。
func (s *Server) snapshotOutlineTree(ctx context.Context, outlineID, note string) {
	if s.outlines == nil || outlineID == "" {
		return
	}
	if _, err := s.outlines.SnapshotTree(ctx, outlineID, note); err != nil {
		if s.logger != nil {
			s.logger.Warn("大纲版本快照失败", "outline_id", outlineID, "error", err)
		}
	}
}
