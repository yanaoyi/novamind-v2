package api

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/yanaoyi/novamindv2/backend/internal/ai"
	"github.com/yanaoyi/novamindv2/backend/internal/domain"
	"github.com/yanaoyi/novamindv2/backend/internal/service"
)

// ---------- 请求 DTO ----------

type createOriginalRequest struct {
	Title       string `json:"title" binding:"required" example:"暗涌"`
	Author      string `json:"author" example:"某作者"`
	Description string `json:"description" example:"导入的原著简介"`
}

// ---------- 响应 DTO ----------

// OriginalResponse 是原著对外表示。
type OriginalResponse struct {
	ID           string    `json:"id"`
	ProjectID    string    `json:"project_id"`
	Title        string    `json:"title"`
	Author       string    `json:"author"`
	Description  string    `json:"description"`
	SourceType   string    `json:"source_type"`
	Status       string    `json:"status"`
	CharCount    int64     `json:"char_count"`
	ChapterCount int       `json:"chapter_count"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// OriginalChapterBrief 是章节目录项（不含正文）。
type OriginalChapterBrief struct {
	ChapterNo int    `json:"chapter_no"`
	Title     string `json:"title"`
	CharCount int    `json:"char_count"`
	Summary   string `json:"summary"`
}

// OriginalChapterDetail 是单章详情（含正文）。
type OriginalChapterDetail struct {
	OriginalChapterBrief
	Content       string `json:"content"`
	StartPosition int64  `json:"start_position"`
	EndPosition   int64  `json:"end_position"`
}

// OriginalChapterList 是章节目录响应。
type OriginalChapterList struct {
	Items    []OriginalChapterBrief `json:"items"`
	Total    int64                  `json:"total"`
	Page     int                    `json:"page"`
	PageSize int                    `json:"page_size"`
}

// OriginalImportResponse 是导入结果。
type OriginalImportResponse struct {
	FileID       string                 `json:"file_id"`
	FileName     string                 `json:"file_name"`
	SizeBytes    int64                  `json:"size_bytes"`
	Encoding     string                 `json:"encoding"`
	CharCount    int64                  `json:"char_count"`
	ChapterCount int                    `json:"chapter_count"`
	Chapters     []service.ChapterBrief `json:"chapters"`
}

func toOriginalResponse(w domain.OriginalWork) OriginalResponse {
	return OriginalResponse{
		ID:           w.ID,
		ProjectID:    w.ProjectID,
		Title:        w.Title,
		Author:       w.Author,
		Description:  w.Description,
		SourceType:   string(w.SourceType),
		Status:       string(w.Status),
		CharCount:    w.CharCount,
		ChapterCount: w.ChapterCount,
		CreatedAt:    w.CreatedAt,
		UpdatedAt:    w.UpdatedAt,
	}
}

// ---------- Handlers ----------

// createOriginal 为工程创建原著。
//
//	@Summary		创建原著
//	@Description	为指定工程创建原著；工程必须是 ORIGINAL 类型，且一个工程只能有一部原著
//	@Tags			original
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string					true	"工程 ID"
//	@Param			body	body		createOriginalRequest	true	"原著信息"
//	@Success		201		{object}	Envelope
//	@Failure		400		{object}	Envelope
//	@Failure		404		{object}	Envelope
//	@Failure		409		{object}	Envelope
//	@Router			/projects/{id}/original [post]
func (s *Server) createOriginal(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	var req createOriginalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "请求参数不合法: "+err.Error(), nil)
		return
	}
	work, err := s.originals.Create(c.Request.Context(), service.CreateOriginalInput{
		ProjectID:   c.Param("id"),
		Title:       req.Title,
		Author:      req.Author,
		Description: req.Description,
	})
	if err != nil {
		s.failFromError(c, err)
		return
	}
	Created(c, toOriginalResponse(*work))
}

// getOriginal 原著详情。
//
//	@Summary	原著详情
//	@Tags		original
//	@Produce	json
//	@Param		id	path		string	true	"原著 ID"
//	@Success	200	{object}	Envelope
//	@Failure	404	{object}	Envelope
//	@Router		/original/{id} [get]
func (s *Server) getOriginal(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	work, err := s.originals.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		s.failFromError(c, err)
		return
	}
	OK(c, toOriginalResponse(*work))
}

// importOriginal 上传原文并解析导入。
//
//	@Summary		导入原著正文
//	@Description	上传 TXT / DOCX 文件，自动探测编码、识别章节并入库。重复导入会整体替换章节。
//	@Tags			original
//	@Accept			multipart/form-data
//	@Produce		json
//	@Param			id		path		string	true	"原著 ID"
//	@Param			file	formData	file	true	"原著文件（txt/docx）"
//	@Success		200		{object}	Envelope
//	@Failure		400		{object}	Envelope
//	@Failure		404		{object}	Envelope
//	@Router			/original/{id}/import [post]
func (s *Server) importOriginal(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	fileHeader, err := c.FormFile("file")
	if err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "缺少上传文件（字段名应为 file）", nil)
		return
	}
	f, err := fileHeader.Open()
	if err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "无法读取上传文件: "+err.Error(), nil)
		return
	}
	defer f.Close()

	result, err := s.originals.Import(c.Request.Context(), c.Param("id"), fileHeader.Filename, f)
	if err != nil {
		s.failFromError(c, err)
		return
	}

	OK(c, OriginalImportResponse{
		FileID:       result.FileID,
		FileName:     result.FileName,
		SizeBytes:    result.SizeBytes,
		Encoding:     result.Encoding,
		CharCount:    result.CharCount,
		ChapterCount: result.ChapterCount,
		Chapters:     result.Chapters,
	})
}

// listOriginalChapters 章节目录。
//
//	@Summary	章节目录
//	@Tags		original
//	@Produce	json
//	@Param		id			path		string	true	"原著 ID"
//	@Param		page		query		int		false	"页码，默认 1"
//	@Param		page_size	query		int		false	"每页条数，默认 50，最大 200"
//	@Success	200			{object}	Envelope
//	@Failure	404			{object}	Envelope
//	@Router		/original/{id}/chapters [get]
func (s *Server) listOriginalChapters(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	page := parseInt(c.Query("page"), 1)
	pageSize := parseInt(c.Query("page_size"), 50)

	items, total, err := s.originals.ListChapters(c.Request.Context(), c.Param("id"), page, pageSize)
	if err != nil {
		s.failFromError(c, err)
		return
	}
	resp := OriginalChapterList{
		Items:    make([]OriginalChapterBrief, 0, len(items)),
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}
	for _, c2 := range items {
		resp.Items = append(resp.Items, OriginalChapterBrief{
			ChapterNo: c2.ChapterNo,
			Title:     c2.Title,
			CharCount: c2.CharCount,
			Summary:   c2.Summary,
		})
	}
	OK(c, resp)
}

// getOriginalChapter 单章正文。
//
//	@Summary	章节正文
//	@Tags		original
//	@Produce	json
//	@Param		id	path		string	true	"原著 ID"
//	@Param		no	path		int		true	"章节序号（从 1 开始）"
//	@Success	200	{object}	Envelope
//	@Failure	404	{object}	Envelope
//	@Router		/original/{id}/chapters/{no} [get]
func (s *Server) getOriginalChapter(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	no, err := strconv.Atoi(c.Param("no"))
	if err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "章节序号必须是整数", nil)
		return
	}
	chapter, err := s.originals.GetChapter(c.Request.Context(), c.Param("id"), no)
	if err != nil {
		s.failFromError(c, err)
		return
	}
	OK(c, OriginalChapterDetail{
		OriginalChapterBrief: OriginalChapterBrief{
			ChapterNo: chapter.ChapterNo,
			Title:     chapter.Title,
			CharCount: chapter.CharCount,
			Summary:   chapter.Summary,
		},
		Content:       chapter.Content,
		StartPosition: chapter.StartPosition,
		EndPosition:   chapter.EndPosition,
	})
}

// failFromOriginalError 是原著相关错误的统一翻译（并入 failFromError 使用）。
func originalErrorStatus(err error) (int, string, bool) {
	switch {
	case errors.Is(err, domain.ErrOriginalNotFound):
		return http.StatusNotFound, "ORIGINAL_NOT_FOUND", true
	case errors.Is(err, domain.ErrChapterNotFound):
		return http.StatusNotFound, "CHAPTER_NOT_FOUND", true
	case errors.Is(err, domain.ErrCharacterNotFound):
		return http.StatusNotFound, "CHARACTER_NOT_FOUND", true
	case errors.Is(err, domain.ErrRelationNotFound):
		return http.StatusNotFound, "RELATION_NOT_FOUND", true
	case errors.Is(err, domain.ErrWorldNotFound):
		return http.StatusNotFound, "WORLD_NOT_FOUND", true
	case errors.Is(err, domain.ErrWorldRuleNotFound):
		return http.StatusNotFound, "WORLD_RULE_NOT_FOUND", true
	case errors.Is(err, domain.ErrLocationNotFound):
		return http.StatusNotFound, "LOCATION_NOT_FOUND", true
	case errors.Is(err, domain.ErrFactionNotFound):
		return http.StatusNotFound, "FACTION_NOT_FOUND", true
	case errors.Is(err, domain.ErrEventNotFound):
		return http.StatusNotFound, "EVENT_NOT_FOUND", true
	case errors.Is(err, domain.ErrPlotArcNotFound):
		return http.StatusNotFound, "PLOT_ARC_NOT_FOUND", true
	case errors.Is(err, domain.ErrProviderNotFound):
		return http.StatusNotFound, "MODEL_PROVIDER_NOT_FOUND", true
	case errors.Is(err, domain.ErrProviderDuplicate):
		return http.StatusConflict, CodeConflict, true
	case errors.Is(err, domain.ErrTaskNotFound):
		return http.StatusNotFound, "TASK_NOT_FOUND", true
	case errors.Is(err, domain.ErrTaskNotCancellable), errors.Is(err, domain.ErrTaskNotRetryable):
		return http.StatusConflict, CodeConflict, true
	case errors.Is(err, domain.ErrProposalNotFound):
		return http.StatusNotFound, "PROPOSAL_NOT_FOUND", true
	case errors.Is(err, domain.ErrProposalAlreadyDecided):
		return http.StatusConflict, CodeConflict, true
	case errors.Is(err, domain.ErrCreativeNotFound),
		errors.Is(err, domain.ErrCreativeCharacterNotFound),
		errors.Is(err, domain.ErrSourceCharacterNotFound),
		errors.Is(err, domain.ErrMappingNotFound):
		return http.StatusNotFound, "CREATIVE_NOT_FOUND", true
	case errors.Is(err, domain.ErrCreativeWorldNotFound),
		errors.Is(err, domain.ErrCreativeWorldRuleNotFound),
		errors.Is(err, domain.ErrDivergenceNotFound),
		errors.Is(err, domain.ErrVolumeNotFound),
		errors.Is(err, domain.ErrCreativeChapterNotFound),
		errors.Is(err, domain.ErrChapterVersionNotFound),
		errors.Is(err, domain.ErrSceneNotFound),
		errors.Is(err, domain.ErrIssueNotFound):
		return http.StatusNotFound, "CREATIVE_WORLD_NOT_FOUND", true
	case errors.Is(err, domain.ErrCreativeAlreadyExists),
		errors.Is(err, domain.ErrCreativeCharacterDup),
		errors.Is(err, domain.ErrCreativeCharacterLocked),
		errors.Is(err, domain.ErrCreativeWorldRuleDup):
		return http.StatusConflict, CodeConflict, true
	case errors.Is(err, domain.ErrOriginalAlreadyExists),
		errors.Is(err, domain.ErrCharacterDuplicate),
		errors.Is(err, domain.ErrRelationDuplicate),
		errors.Is(err, domain.ErrWorldRuleDuplicate),
		errors.Is(err, domain.ErrLocationDuplicate),
		errors.Is(err, domain.ErrFactionDuplicate):
		return http.StatusConflict, CodeConflict, true
	case errors.Is(err, domain.ErrOriginalNotOriginalProj),
		errors.Is(err, domain.ErrOriginalTitleRequired),
		errors.Is(err, domain.ErrOriginalTitleTooLong),
		errors.Is(err, domain.ErrOriginalStatusInvalid),
		errors.Is(err, domain.ErrImportSourceInvalid),
		errors.Is(err, domain.ErrImportEmpty),
		errors.Is(err, domain.ErrCharacterNameRequired),
		errors.Is(err, domain.ErrCharacterNameTooLong),
		errors.Is(err, domain.ErrCharacterImportance),
		errors.Is(err, domain.ErrDNAWeightOutOfRange),
		errors.Is(err, domain.ErrRelationTypeInvalid),
		errors.Is(err, domain.ErrRelationSelfReference),
		errors.Is(err, domain.ErrRelationStrength),
		errors.Is(err, domain.ErrRelationCrossWork),
		errors.Is(err, domain.ErrWorldRuleNameEmpty),
		errors.Is(err, domain.ErrWorldRuleImportance),
		errors.Is(err, domain.ErrLocationNameEmpty),
		errors.Is(err, domain.ErrLocationParentCross),
		errors.Is(err, domain.ErrLocationSelfParent),
		errors.Is(err, domain.ErrLocationCycle),
		errors.Is(err, domain.ErrFactionNameEmpty),
		errors.Is(err, domain.ErrEventTitleEmpty),
		errors.Is(err, domain.ErrEventImportance),
		errors.Is(err, domain.ErrEventChapterNo),
		errors.Is(err, domain.ErrEventParticipant),
		errors.Is(err, domain.ErrTimelineOrderBad),
		errors.Is(err, domain.ErrPlotArcTitleEmpty),
		errors.Is(err, domain.ErrPlotArcTypeInvalid),
		errors.Is(err, domain.ErrProviderNameEmpty),
		errors.Is(err, domain.ErrProviderTypeInvalid),
		errors.Is(err, domain.ErrProviderBaseEmpty),
		errors.Is(err, domain.ErrProviderModelEmpty),
		errors.Is(err, domain.ErrProviderTempRange),
		errors.Is(err, domain.ErrProviderPurposeBad),
		errors.Is(err, domain.ErrNoProviderAvailable),
		errors.Is(err, ai.ErrMissingSecret),
		errors.Is(err, domain.ErrTaskTypeEmpty),
		errors.Is(err, domain.ErrTaskStatusInvalid),
		errors.Is(err, service.ErrTaskTypeUnknown),
		errors.Is(err, domain.ErrProposalStageInvalid),
		errors.Is(err, domain.ErrProposalEntityInvalid),
		errors.Is(err, domain.ErrProposalPayloadInvalid),
		errors.Is(err, service.ErrModelJSONInvalid),
		errors.Is(err, service.ErrNoChapters),
		errors.Is(err, domain.ErrCreativeTitleEmpty),
		errors.Is(err, domain.ErrCreativeNotCreativeProj),
		errors.Is(err, domain.ErrCreativeCharacterName),
		errors.Is(err, domain.ErrSourceCharacterCrossWork),
		errors.Is(err, domain.ErrInheritanceWeightInvalid),
		errors.Is(err, domain.ErrInheritanceNoDimension),
		errors.Is(err, domain.ErrFusionNeedsTwoSources),
		errors.Is(err, domain.ErrCreativeWorldModeBad),
		errors.Is(err, domain.ErrCreativeWorldRuleName),
		errors.Is(err, domain.ErrCreativeWorldRuleStatus),
		errors.Is(err, domain.ErrDivergenceSourceInvalid),
		errors.Is(err, domain.ErrCreativeTimelineBad),
		errors.Is(err, domain.ErrVolumeTitleEmpty),
		errors.Is(err, domain.ErrChapterTitleEmpty),
		errors.Is(err, domain.ErrChapterNoInvalid),
		errors.Is(err, domain.ErrChapterStatusInvalid),
		errors.Is(err, domain.ErrIssueSeverityInvalid),
		errors.Is(err, domain.ErrIssueTypeInvalid),
		errors.Is(err, domain.ErrIssueStatusInvalid),
		errors.Is(err, domain.ErrExportFormatInvalid),
		errors.Is(err, service.ErrUploadTooLarge):
		return http.StatusBadRequest, CodeBadRequest, true
	default:
		return 0, "", false
	}
}
