package api

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/yanaoyi/novamindv2/backend/internal/domain"
	"github.com/yanaoyi/novamindv2/backend/internal/repository"
	"github.com/yanaoyi/novamindv2/backend/internal/service"
)

// ---------- 请求 DTO ----------

type createProjectRequest struct {
	Name        string `json:"name" binding:"required" example:"人间真相"`
	Description string `json:"description" example:"六个普通人的群像故事"`
	Type        string `json:"type" binding:"required,oneof=ORIGINAL CREATIVE" example:"CREATIVE"`
}

type updateProjectRequest struct {
	Name        *string `json:"name" example:"人间真相（修订）"`
	Description *string `json:"description" example:"新的简介"`
	Status      *string `json:"status" binding:"omitempty,oneof=DRAFT ACTIVE ARCHIVED" example:"ACTIVE"`
}

// ---------- 响应 DTO ----------

// ProjectResponse 是文章的对外表示。
type ProjectResponse struct {
	ID          string    `json:"id" example:"01923456-789a-7bcd-ef01-23456789abcd"`
	Name        string    `json:"name" example:"人间真相"`
	Description string    `json:"description" example:"六个普通人的群像故事"`
	Type        string    `json:"type" example:"CREATIVE"`
	Status      string    `json:"status" example:"ACTIVE"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ProjectListResponse 是分页列表响应。
type ProjectListResponse struct {
	Items    []ProjectResponse `json:"items"`
	Total    int64             `json:"total"`
	Page     int               `json:"page"`
	PageSize int               `json:"page_size"`
}

func toProjectResponse(p domain.Project) ProjectResponse {
	return ProjectResponse{
		ID:          p.ID,
		Name:        p.Name,
		Description: p.Description,
		Type:        string(p.Type),
		Status:      string(p.Status),
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
	}
}

// ---------- Handlers ----------

// createProject 创建文章。
//
//	@Summary		创建文章
//	@Description	创建一个原著（ORIGINAL）或二创（CREATIVE）文章
//	@Tags			projects
//	@Accept			json
//	@Produce		json
//	@Param			body	body		createProjectRequest	true	"文章信息"
//	@Success		201		{object}	Envelope
//	@Failure		400		{object}	Envelope
//	@Failure		500		{object}	Envelope
//	@Router			/projects [post]
func (s *Server) createProject(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	var req createProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "请求参数不合法: "+err.Error(), nil)
		return
	}

	p, err := s.projects.Create(c.Request.Context(), service.CreateProjectInput{
		Name:        req.Name,
		Description: req.Description,
		Type:        domain.ProjectType(req.Type),
	})
	if err != nil {
		s.failFromError(c, err)
		return
	}
	Created(c, toProjectResponse(*p))
}

// listProjects 分页查询文章。
//
//	@Summary		文章列表
//	@Description	分页查询文章，可按类型、状态过滤，按名称模糊搜索
//	@Tags			projects
//	@Produce		json
//	@Param			page		query		int		false	"页码，默认 1"
//	@Param			page_size	query		int		false	"每页条数，默认 20，最大 100"
//	@Param			type		query		string	false	"文章类型"	Enums(ORIGINAL, CREATIVE)
//	@Param			status		query		string	false	"文章状态"	Enums(DRAFT, ACTIVE, ARCHIVED)
//	@Param			keyword		query		string	false	"名称关键字"
//	@Success		200			{object}	Envelope
//	@Failure		400			{object}	Envelope
//	@Router			/projects [get]
func (s *Server) listProjects(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	filter := repository.ProjectFilter{
		Page:     parseInt(c.Query("page"), 1),
		PageSize: parseInt(c.Query("page_size"), 20),
		Keyword:  c.Query("keyword"),
	}
	if v := c.Query("type"); v != "" {
		t := domain.ProjectType(v)
		if !t.Valid() {
			Fail(c, http.StatusBadRequest, CodeBadRequest, "type 必须是 ORIGINAL 或 CREATIVE", nil)
			return
		}
		filter.Type = &t
	}
	if v := c.Query("status"); v != "" {
		st := domain.ProjectStatus(v)
		if !st.Valid() {
			Fail(c, http.StatusBadRequest, CodeBadRequest, "status 必须是 DRAFT / ACTIVE / ARCHIVED", nil)
			return
		}
		filter.Status = &st
	}

	items, total, err := s.projects.List(c.Request.Context(), filter)
	if err != nil {
		s.failFromError(c, err)
		return
	}
	resp := ProjectListResponse{
		Items:    make([]ProjectResponse, 0, len(items)),
		Total:    total,
		Page:     filter.Page,
		PageSize: filter.PageSize,
	}
	for _, it := range items {
		resp.Items = append(resp.Items, toProjectResponse(it))
	}
	OK(c, resp)
}

// getProject 查询单个文章。
//
//	@Summary		文章详情
//	@Tags			projects
//	@Produce		json
//	@Param			id	path		string	true	"文章 ID（UUID）"
//	@Success		200	{object}	Envelope
//	@Failure		404	{object}	Envelope
//	@Router			/projects/{id} [get]
func (s *Server) getProject(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	p, err := s.projects.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		s.failFromError(c, err)
		return
	}
	OK(c, toProjectResponse(*p))
}

// updateProject 更新文章。
//
//	@Summary		更新文章
//	@Description	更新名称/简介/状态；文章类型不可变更
//	@Tags			projects
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string					true	"文章 ID（UUID）"
//	@Param			body	body		updateProjectRequest	true	"待更新字段（缺省字段不变）"
//	@Success		200		{object}	Envelope
//	@Failure		400		{object}	Envelope
//	@Failure		404		{object}	Envelope
//	@Router			/projects/{id} [put]
func (s *Server) updateProject(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	var req updateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "请求参数不合法: "+err.Error(), nil)
		return
	}

	in := service.UpdateProjectInput{Name: req.Name, Description: req.Description}
	if req.Status != nil {
		st := domain.ProjectStatus(*req.Status)
		in.Status = &st
	}

	p, err := s.projects.Update(c.Request.Context(), c.Param("id"), in)
	if err != nil {
		s.failFromError(c, err)
		return
	}
	OK(c, toProjectResponse(*p))
}

// deleteProject 软删除文章。
//
//	@Summary		删除文章
//	@Description	软删除（写 deleted_at），不物理删除数据
//	@Tags			projects
//	@Produce		json
//	@Param			id	path		string	true	"文章 ID（UUID）"
//	@Success		200	{object}	Envelope
//	@Failure		404	{object}	Envelope
//	@Router			/projects/{id} [delete]
func (s *Server) deleteProject(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	if err := s.projects.Delete(c.Request.Context(), c.Param("id")); err != nil {
		s.failFromError(c, err)
		return
	}
	OK(c, gin.H{"deleted": true})
}

// ---------- 错误翻译 ----------

// failFromError 把领域错误翻译成 HTTP 错误码；未知错误记日志并返回 500。
func (s *Server) failFromError(c *gin.Context, err error) {
	if status, code, ok := originalErrorStatus(err); ok {
		Fail(c, status, code, err.Error(), nil)
		return
	}
	switch {
	case errors.Is(err, domain.ErrProjectNotFound):
		Fail(c, http.StatusNotFound, "PROJECT_NOT_FOUND", err.Error(), nil)
	case errors.Is(err, service.ErrBadRequest):
		Fail(c, http.StatusBadRequest, CodeBadRequest, err.Error(), nil)
	case errors.Is(err, domain.ErrProjectNameRequired),
		errors.Is(err, domain.ErrProjectNameTooLong),
		errors.Is(err, domain.ErrProjectTypeInvalid),
		errors.Is(err, domain.ErrProjectStatusBad):
		Fail(c, http.StatusBadRequest, CodeBadRequest, err.Error(), nil)
	default:
		if s.logger != nil {
			s.logger.Error("请求处理失败",
				slog.String("trace_id", traceID(c)),
				slog.String("path", c.Request.URL.Path),
				slog.Any("error", err),
			)
		}
		Fail(c, http.StatusInternalServerError, CodeInternal, "服务器内部错误", nil)
	}
}

func parseInt(raw string, fallback int) int {
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return v
}
