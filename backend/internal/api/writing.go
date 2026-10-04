package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/yanaoyi/novamindv2/backend/internal/domain"
	"github.com/yanaoyi/novamindv2/backend/internal/service"
)

// ---------- 请求 DTO ----------

type createVolumeRequest struct {
	Title    string `json:"title" binding:"required"`
	Summary  string `json:"summary"`
	Sequence int    `json:"sequence"`
}

type createChapterRequest struct {
	VolumeID  *string `json:"volume_id"`
	ChapterNo int     `json:"chapter_no" binding:"required"`
	Title     string  `json:"title" binding:"required"`
	Summary   string  `json:"summary"`
	Purpose   string  `json:"purpose"`
	Conflict  string  `json:"conflict"`
	Outcome   string  `json:"outcome"`
	Content   string  `json:"content"`
}

type updateChapterRequest struct {
	Title    *string `json:"title"`
	Summary  *string `json:"summary"`
	Content  *string `json:"content"`
	Status   *string `json:"status" binding:"omitempty,oneof=DRAFT REVIEW FINAL"`
	VolumeID *string `json:"volume_id"`
	Purpose  *string `json:"purpose"`
	Conflict *string `json:"conflict"`
	Outcome  *string `json:"outcome"`
}

type generateChapterRequest struct {
	TargetWords int    `json:"target_words" example:"2000"`
	Instruction string `json:"instruction"`
}

type rewriteRequest struct {
	ChapterID   string `json:"chapter_id" binding:"required"`
	Text        string `json:"text" binding:"required"`
	Action      string `json:"action" binding:"required,oneof=改写 扩写 缩写 润色 增强冲突 增强情绪"`
	Instruction string `json:"instruction"`
}

type consistencyCheckRequest struct {
	ChapterIDs []string `json:"chapter_ids"`
}

type issueStatusRequest struct {
	Status string `json:"status" binding:"required,oneof=OPEN RESOLVED IGNORED"`
}

type createSceneRequest struct {
	Sequence      int      `json:"sequence"`
	Title         string   `json:"title"`
	Location      string   `json:"location"`
	Characters    []string `json:"characters"`
	Purpose       string   `json:"purpose"`
	Conflict      string   `json:"conflict"`
	EmotionalGoal string   `json:"emotional_goal"`
	Content       string   `json:"content"`
}

// ---------- 响应 DTO ----------

// ChapterResponse 是章节对外表示。
type ChapterResponse struct {
	ID        string    `json:"id"`
	VolumeID  *string   `json:"volume_id"`
	ChapterNo int       `json:"chapter_no"`
	Title     string    `json:"title"`
	Summary   string    `json:"summary"`
	Content   string    `json:"content"`
	Status    string    `json:"status"`
	WordCount int       `json:"word_count"`
	Purpose   string    `json:"purpose"`
	Conflict  string    `json:"conflict"`
	Outcome   string    `json:"outcome"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func toChapterResponse(c domain.CreativeChapter) ChapterResponse {
	return ChapterResponse{
		ID: c.ID, VolumeID: c.VolumeID, ChapterNo: c.ChapterNo, Title: c.Title, Summary: c.Summary,
		Content: c.Content, Status: string(c.Status), WordCount: c.WordCount,
		Purpose: c.Purpose, Conflict: c.Conflict, Outcome: c.Outcome,
		CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
	}
}

// VolumeResponse 是卷对外表示。
type VolumeResponse struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Summary   string    `json:"summary"`
	Sequence  int       `json:"sequence"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func toVolumeResponse(v domain.CreativeVolume) VolumeResponse {
	return VolumeResponse{
		ID: v.ID, Title: v.Title, Summary: v.Summary, Sequence: v.Sequence,
		CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt,
	}
}

// ChapterVersionResponse 是章节版本对外表示（列表默认不含正文）。
type ChapterVersionResponse struct {
	ID        string    `json:"id"`
	ChapterID string    `json:"chapter_id"`
	VersionNo int       `json:"version_no"`
	Content   string    `json:"content,omitempty"`
	WordCount int       `json:"word_count"`
	Note      string    `json:"note"`
	CreatedAt time.Time `json:"created_at"`
}

func toChapterVersionResponse(v domain.ChapterVersion, withContent bool) ChapterVersionResponse {
	out := ChapterVersionResponse{
		ID: v.ID, ChapterID: v.ChapterID, VersionNo: v.VersionNo,
		WordCount: v.WordCount, Note: v.Note, CreatedAt: v.CreatedAt,
	}
	if withContent {
		out.Content = v.Content
	}
	return out
}

// SceneResponse 是场景对外表示。
type SceneResponse struct {
	ID            string    `json:"id"`
	ChapterID     string    `json:"chapter_id"`
	Sequence      int       `json:"sequence"`
	Title         string    `json:"title"`
	Location      string    `json:"location"`
	Characters    []string  `json:"characters"`
	Purpose       string    `json:"purpose"`
	Conflict      string    `json:"conflict"`
	EmotionalGoal string    `json:"emotional_goal"`
	Content       string    `json:"content"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func toSceneResponse(s domain.CreativeScene) SceneResponse {
	characters := s.Characters
	if characters == nil {
		characters = []string{}
	}
	return SceneResponse{
		ID: s.ID, ChapterID: s.ChapterID, Sequence: s.Sequence, Title: s.Title,
		Location: s.Location, Characters: characters, Purpose: s.Purpose,
		Conflict: s.Conflict, EmotionalGoal: s.EmotionalGoal, Content: s.Content,
		CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt,
	}
}

// ConsistencyIssueResponse 是一致性问题对外表示。
type ConsistencyIssueResponse struct {
	ID             string    `json:"id"`
	CreativeWorkID string    `json:"creative_work_id"`
	ChapterID      *string   `json:"chapter_id"`
	Severity       string    `json:"severity"`
	Type           string    `json:"type"`
	Description    string    `json:"description"`
	Evidence       string    `json:"evidence"`
	Suggestion     string    `json:"suggestion"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func toIssueResponse(i domain.ConsistencyIssue) ConsistencyIssueResponse {
	return ConsistencyIssueResponse{
		ID: i.ID, CreativeWorkID: i.CreativeWorkID, ChapterID: i.ChapterID,
		Severity: i.Severity, Type: i.Type, Description: i.Description,
		Evidence: i.Evidence, Suggestion: i.Suggestion, Status: i.Status,
		CreatedAt: i.CreatedAt, UpdatedAt: i.UpdatedAt,
	}
}

// ---------- Handlers ----------

// createVolume 新增卷。
//
//	@Summary	新增卷
//	@Tags		writing
//	@Accept		json
//	@Produce	json
//	@Param		id		path		string				true	"二创作品 ID"
//	@Param		body	body		createVolumeRequest	true	"卷"
//	@Success	201		{object}	Envelope
//	@Router		/creative/{id}/volumes [post]
func (s *Server) createVolume(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	var req createVolumeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "请求参数不合法: "+err.Error(), nil)
		return
	}
	v, err := s.writing.CreateVolume(c.Request.Context(), c.Param("id"), service.CreateVolumeInput{
		Title: req.Title, Summary: req.Summary, Sequence: req.Sequence,
	})
	if err != nil {
		s.failFromError(c, err)
		return
	}
	s.snapshotOutline(c.Request.Context(), c.Param("id"), "新增卷")
	Created(c, gin.H{"id": v.ID, "title": v.Title, "summary": v.Summary, "sequence": v.Sequence})
}

// listVolumes 卷列表。
//
//	@Summary	卷列表
//	@Tags		writing
//	@Produce	json
//	@Param		id	path		string	true	"二创作品 ID"
//	@Success	200	{object}	Envelope
//	@Router		/creative/{id}/volumes [get]
func (s *Server) listVolumes(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	items, err := s.writing.ListVolumes(c.Request.Context(), c.Param("id"))
	if err != nil {
		s.failFromError(c, err)
		return
	}
	resp := make([]VolumeResponse, 0, len(items))
	for _, v := range items {
		resp = append(resp, toVolumeResponse(v))
	}
	OK(c, gin.H{"items": resp, "total": len(resp)})
}

// createChapter 新增章节。
//
//	@Summary		新增章节
//	@Description	content 非空时会自动生成 v1 版本。
//	@Tags			writing
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string					true	"二创作品 ID"
//	@Param			body	body		createChapterRequest	true	"章节"
//	@Success		201		{object}	Envelope
//	@Failure		400		{object}	Envelope
//	@Router			/creative/{id}/chapters [post]
func (s *Server) createChapter(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	var req createChapterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "请求参数不合法: "+err.Error(), nil)
		return
	}
	chapter, err := s.writing.CreateChapter(c.Request.Context(), c.Param("id"), service.CreateChapterInput{
		VolumeID: req.VolumeID, ChapterNo: req.ChapterNo, Title: req.Title, Summary: req.Summary,
		Purpose: req.Purpose, Conflict: req.Conflict, Outcome: req.Outcome, Content: req.Content,
	})
	if err != nil {
		s.failFromError(c, err)
		return
	}
	s.snapshotOutline(c.Request.Context(), c.Param("id"), "新增章节")
	Created(c, toChapterResponse(*chapter))
}

// listChapters 章节列表（默认不含正文）。
//
//	@Summary	章节列表
//	@Tags		writing
//	@Produce	json
//	@Param		id		path		string	true	"二创作品 ID"
//	@Param		full	query		bool	false	"是否包含正文"
//	@Success	200		{object}	Envelope
//	@Router		/creative/{id}/chapters [get]
func (s *Server) listCreativeChapters(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	withContent := c.Query("full") == "true"
	items, err := s.writing.ListChapters(c.Request.Context(), c.Param("id"), withContent)
	if err != nil {
		s.failFromError(c, err)
		return
	}
	resp := make([]ChapterResponse, 0, len(items))
	for _, c2 := range items {
		resp = append(resp, toChapterResponse(c2))
	}
	OK(c, gin.H{"items": resp, "total": len(resp)})
}

// getCreativeChapter 章节详情。
//
//	@Summary	章节详情
//	@Tags		writing
//	@Produce	json
//	@Param		id	path		string	true	"章节 ID"
//	@Success	200	{object}	Envelope
//	@Failure	404	{object}	Envelope
//	@Router		/chapters/{id} [get]
func (s *Server) getCreativeChapter(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	chapter, err := s.writing.GetChapter(c.Request.Context(), c.Param("id"))
	if err != nil {
		s.failFromError(c, err)
		return
	}
	OK(c, toChapterResponse(*chapter))
}

// updateCreativeChapter 更新章节。
//
//	@Summary		更新章节
//	@Description	正文有变化时会自动存一个新版本（规格书 §59）。
//	@Tags			writing
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string					true	"章节 ID"
//	@Param			body	body		updateChapterRequest	true	"待更新字段"
//	@Success		200		{object}	Envelope
//	@Failure		404		{object}	Envelope
//	@Router			/chapters/{id} [put]
func (s *Server) updateCreativeChapter(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	var req updateChapterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "请求参数不合法: "+err.Error(), nil)
		return
	}
	in := service.UpdateChapterInput{
		Title: req.Title, Summary: req.Summary, Content: req.Content, VolumeID: req.VolumeID,
		Purpose: req.Purpose, Conflict: req.Conflict, Outcome: req.Outcome,
	}
	if req.Status != nil {
		status := domain.ChapterStatus(*req.Status)
		in.Status = &status
	}
	chapter, versioned, err := s.writing.UpdateChapter(c.Request.Context(), c.Param("id"), in)
	if err != nil {
		s.failFromError(c, err)
		return
	}
	s.snapshotOutline(c.Request.Context(), chapter.CreativeWorkID, "更新章节大纲")
	resp := toChapterResponse(*chapter)
	OK(c, gin.H{"chapter": resp, "version_created": versioned})
}

// deleteCreativeChapter 删除章节。
//
//	@Summary	删除章节
//	@Tags		writing
//	@Produce	json
//	@Param		id	path		string	true	"章节 ID"
//	@Success	200	{object}	Envelope
//	@Router		/chapters/{id} [delete]
func (s *Server) deleteCreativeChapter(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	if err := s.writing.DeleteChapter(c.Request.Context(), c.Param("id")); err != nil {
		s.failFromError(c, err)
		return
	}
	OK(c, gin.H{"deleted": true})
}

// generateChapter 让 AI 写本章（异步）。
//
//	@Summary		让 AI 写本章
//	@Description	按人物 DNA、世界规则与最近章节摘要组装上下文，异步生成正文并自动存版本。
//	@Tags			writing
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string					true	"章节 ID"
//	@Param			body	body		generateChapterRequest	false	"目标字数与补充要求"
//	@Success		202		{object}	Envelope
//	@Failure		404		{object}	Envelope
//	@Router			/chapters/{id}/generate [post]
func (s *Server) generateChapter(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	chapter, err := s.writing.GetChapter(c.Request.Context(), c.Param("id"))
	if err != nil {
		s.failFromError(c, err)
		return
	}
	var req generateChapterRequest
	if err := bindOptionalJSON(c, &req); err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, err.Error(), nil)
		return
	}

	input := map[string]any{"chapter_id": chapter.ID, "target_words": req.TargetWords, "instruction": req.Instruction}
	t, err := s.tasks.Enqueue(c.Request.Context(), service.EnqueueInput{
		Type:           "writing_chapter",
		CreativeWorkID: &chapter.CreativeWorkID,
		Input:          input,
	})
	if err != nil {
		s.failFromError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, Envelope{Data: toTaskResponse(domainTask(*t)), TraceID: traceID(c)})
}

// listChapterVersions 章节版本列表。
//
//	@Summary	章节版本列表
//	@Tags		writing
//	@Produce	json
//	@Param		id	path		string	true	"章节 ID"
//	@Success	200	{object}	Envelope
//	@Router		/chapters/{id}/versions [get]
func (s *Server) listChapterVersions(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	items, err := s.writing.ListVersions(c.Request.Context(), c.Param("id"))
	if err != nil {
		s.failFromError(c, err)
		return
	}
	resp := make([]ChapterVersionResponse, 0, len(items))
	for _, v := range items {
		resp = append(resp, toChapterVersionResponse(v, false))
	}
	OK(c, gin.H{"items": resp, "total": len(resp)})
}

// getChapterVersion 取某个版本（含正文）。
//
//	@Summary	章节版本详情
//	@Tags		writing
//	@Produce	json
//	@Param		id	path		string	true	"章节 ID"
//	@Param		no	path		int		true	"版本号"
//	@Success	200	{object}	Envelope
//	@Failure	404	{object}	Envelope
//	@Router		/chapters/{id}/versions/{no} [get]
func (s *Server) getChapterVersion(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	no, err := parseIntParam(c, "no")
	if err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "版本号必须是整数", nil)
		return
	}
	version, err := s.writing.GetVersion(c.Request.Context(), c.Param("id"), no)
	if err != nil {
		s.failFromError(c, err)
		return
	}
	OK(c, toChapterVersionResponse(*version, true))
}

// restoreChapterVersion 恢复到某个版本。
//
//	@Summary		恢复章节版本
//	@Description	恢复前会先把当前正文自动存一版，避免误操作丢内容。
//	@Tags			writing
//	@Produce		json
//	@Param			id	path		string	true	"章节 ID"
//	@Param			no	path		int		true	"版本号"
//	@Success		200	{object}	Envelope
//	@Router			/chapters/{id}/versions/{no}/restore [post]
func (s *Server) restoreChapterVersion(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	no, err := parseIntParam(c, "no")
	if err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "版本号必须是整数", nil)
		return
	}
	chapter, err := s.writing.RestoreVersion(c.Request.Context(), c.Param("id"), no)
	if err != nil {
		s.failFromError(c, err)
		return
	}
	OK(c, toChapterResponse(*chapter))
}

// createChapterScene 新增场景。
//
//	@Summary	新增场景
//	@Tags		writing
//	@Accept		json
//	@Produce	json
//	@Param		id		path		string					true	"章节 ID"
//	@Param		body	body		domain.CreativeScene	true	"场景"
//	@Success	201		{object}	Envelope
//	@Router		/chapters/{id}/scenes [post]
func (s *Server) createChapterScene(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	var req createSceneRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "请求参数不合法: "+err.Error(), nil)
		return
	}
	created, err := s.writing.CreateScene(c.Request.Context(), c.Param("id"), domain.CreativeScene{
		Sequence: req.Sequence, Title: req.Title, Location: req.Location, Characters: req.Characters,
		Purpose: req.Purpose, Conflict: req.Conflict, EmotionalGoal: req.EmotionalGoal, Content: req.Content,
	})
	if err != nil {
		s.failFromError(c, err)
		return
	}
	Created(c, toSceneResponse(*created))
}

// listChapterScenes 场景列表。
//
//	@Summary	场景列表
//	@Tags		writing
//	@Produce	json
//	@Param		id	path		string	true	"章节 ID"
//	@Success	200	{object}	Envelope
//	@Router		/chapters/{id}/scenes [get]
func (s *Server) listChapterScenes(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	items, err := s.writing.ListScenes(c.Request.Context(), c.Param("id"))
	if err != nil {
		s.failFromError(c, err)
		return
	}
	resp := make([]SceneResponse, 0, len(items))
	for _, s2 := range items {
		resp = append(resp, toSceneResponse(s2))
	}
	OK(c, gin.H{"items": resp, "total": len(resp)})
}

// rewriteText 编辑器内的 AI 操作（改写/扩写/缩写/润色/增强冲突/增强情绪，同步返回）。
//
//	@Summary		编辑器 AI 操作
//	@Tags			writing
//	@Accept			json
//	@Produce		json
//	@Param			body	body		rewriteRequest	true	"选中文本与操作类型"
//	@Success		200		{object}	Envelope
//	@Failure		400		{object}	Envelope
//	@Router			/ai/rewrite [post]
func (s *Server) rewriteText(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	if s.invoker == nil {
		Fail(c, http.StatusServiceUnavailable, "MODEL_NOT_READY", "模型未配置", nil)
		return
	}
	var req rewriteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "请求参数不合法: "+err.Error(), nil)
		return
	}
	result, err := s.writing.RewriteText(c.Request.Context(), s.invoker, service.RewriteInput{
		ChapterID: req.ChapterID, Text: req.Text,
		Action: service.RewriteAction(req.Action), Instruction: req.Instruction,
	})
	if err != nil {
		s.failFromError(c, err)
		return
	}
	OK(c, gin.H{"text": result})
}

// checkConsistency 触发一致性检查（异步）。
//
//	@Summary		一致性检查（异步）
//	@Description	不传 chapter_ids 时检查全部有正文的章节。结果进「一致性问题」列表。
//	@Tags			consistency
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string						true	"二创作品 ID"
//	@Param			body	body		consistencyCheckRequest		false	"指定章节"
//	@Success		202		{object}	Envelope
//	@Router			/creative/{id}/consistency/check [post]
func (s *Server) checkConsistency(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	var req consistencyCheckRequest
	if err := bindOptionalJSON(c, &req); err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, err.Error(), nil)
		return
	}
	workID := c.Param("id")
	if _, err := s.creative.GetWork(c.Request.Context(), workID); err != nil {
		s.failFromError(c, err)
		return
	}
	t, err := s.tasks.Enqueue(c.Request.Context(), service.EnqueueInput{
		Type:           "consistency_check",
		CreativeWorkID: &workID,
		Input:          map[string]any{"work_id": workID, "chapter_ids": req.ChapterIDs},
	})
	if err != nil {
		s.failFromError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, Envelope{Data: toTaskResponse(domainTask(*t)), TraceID: traceID(c)})
}

// listConsistencyIssues 一致性问题列表。
//
//	@Summary	一致性问题列表
//	@Tags		consistency
//	@Produce	json
//	@Param		id		path		string	true	"二创作品 ID"
//	@Param		status	query		string	false	"状态"	Enums(OPEN, RESOLVED, IGNORED)
//	@Success	200		{object}	Envelope
//	@Router		/creative/{id}/consistency/issues [get]
func (s *Server) listConsistencyIssues(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	items, err := s.writing.ListIssues(c.Request.Context(), c.Param("id"), c.Query("status"))
	if err != nil {
		s.failFromError(c, err)
		return
	}
	resp := make([]ConsistencyIssueResponse, 0, len(items))
	for _, i := range items {
		resp = append(resp, toIssueResponse(i))
	}
	OK(c, gin.H{"items": resp, "total": len(resp)})
}

// updateConsistencyIssue 更新问题状态。
//
//	@Summary	更新一致性问题状态
//	@Tags		consistency
//	@Accept		json
//	@Produce	json
//	@Param		id		path		string				true	"问题 ID"
//	@Param		body	body		issueStatusRequest	true	"状态"
//	@Success	200		{object}	Envelope
//	@Router		/consistency-issues/{id} [put]
func (s *Server) updateConsistencyIssue(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	var req issueStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "请求参数不合法: "+err.Error(), nil)
		return
	}
	if err := s.writing.UpdateIssueStatus(c.Request.Context(), c.Param("id"), req.Status); err != nil {
		s.failFromError(c, err)
		return
	}
	OK(c, gin.H{"updated": true})
}

// exportCreative 导出作品（txt / md / docx）。
//
//	@Summary		导出作品
//	@Description	支持 txt / md / docx，按「卷 → 章」结构输出。
//	@Tags			writing
//	@Produce		application/octet-stream
//	@Param			id		path		string	true	"二创作品 ID"
//	@Param			format	query		string	false	"导出格式"	Enums(txt, md, docx)
//	@Success		200		{file}		binary
//	@Router			/creative/{id}/export [get]
func (s *Server) exportCreative(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	data, filename, contentType, err := s.writing.Export(c.Request.Context(), c.Param("id"), c.Query("format"))
	if err != nil {
		s.failFromError(c, err)
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename*=UTF-8''%s", escapeFilename(filename)))
	c.Data(http.StatusOK, contentType, data)
}

func parseIntParam(c *gin.Context, name string) (int, error) {
	raw := c.Param(name)
	var value int
	if _, err := fmt.Sscanf(raw, "%d", &value); err != nil {
		return 0, err
	}
	return value, nil
}

func escapeFilename(name string) string {
	out := ""
	for _, r := range name {
		if r < 128 && r != ' ' && r != '"' {
			out += string(r)
		} else {
			out += fmt.Sprintf("%%%02X", []byte(string(r)))
		}
	}
	return out
}
