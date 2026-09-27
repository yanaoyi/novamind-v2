package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/yanaoyi/novamindv2/backend/internal/domain"
	"github.com/yanaoyi/novamindv2/backend/internal/repository"
	"github.com/yanaoyi/novamindv2/backend/internal/service"
)

// domainTask 是 domain.Task 的别名（避免与包内其它标识混淆）。
type domainTask = domain.Task

// 任务类型请求（手工入队，供调试与将来扩展）。
type createTaskRequest struct {
	Type           string         `json:"type" binding:"required" example:"original_reparse"`
	ProjectID      *string        `json:"project_id"`
	WorkID         *string        `json:"work_id"`
	CreativeWorkID *string        `json:"creative_work_id"`
	Input          map[string]any `json:"input"`
	MaxAttempts    int            `json:"max_attempts" example:"3"`
}

// TaskResponse 是任务对外表示。
type TaskResponse struct {
	ID              string         `json:"id"`
	ProjectID       *string        `json:"project_id"`
	WorkID          *string        `json:"work_id"`
	CreativeWorkID  *string        `json:"creative_work_id"`
	Type            string         `json:"type"`
	Status          string         `json:"status"`
	Progress        int            `json:"progress"`
	ProgressMessage string         `json:"progress_message"`
	Input           map[string]any `json:"input"`
	Output          map[string]any `json:"output"`
	Error           string         `json:"error"`
	Attempts        int            `json:"attempts"`
	MaxAttempts     int            `json:"max_attempts"`
	CreatedAt       time.Time      `json:"created_at"`
	StartedAt       *time.Time     `json:"started_at"`
	FinishedAt      *time.Time     `json:"finished_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

// TaskListResponse 是任务列表响应。
type TaskListResponse struct {
	Items    []TaskResponse `json:"items"`
	Total    int64          `json:"total"`
	Page     int            `json:"page"`
	PageSize int            `json:"page_size"`
}

func toTaskResponse(t domainTask) TaskResponse {
	return TaskResponse{
		ID: t.ID, ProjectID: t.ProjectID, WorkID: t.WorkID, CreativeWorkID: t.CreativeWorkID, Type: t.Type,
		Status: string(t.Status), Progress: t.Progress, ProgressMessage: t.ProgressMessage,
		Input: t.Input, Output: t.Output, Error: t.Error,
		Attempts: t.Attempts, MaxAttempts: t.MaxAttempts,
		CreatedAt: t.CreatedAt, StartedAt: t.StartedAt, FinishedAt: t.FinishedAt, UpdatedAt: t.UpdatedAt,
	}
}

// listTasks 任务列表。
//
//	@Summary	任务列表
//	@Tags		tasks
//	@Produce	json
//	@Param		project_id	query		string	false	"按工程过滤"
//	@Param		work_id		query		string	false	"按原著过滤"
//	@Param		creative_work_id	query		string	false	"按二创作品过滤"
//	@Param		status		query		string	false	"按状态过滤"	Enums(PENDING, RUNNING, PAUSED, COMPLETED, FAILED, CANCELLED)
//	@Param		type		query		string	false	"按类型过滤"
//	@Param		page		query		int		false	"页码"
//	@Param		page_size	query		int		false	"每页条数，默认 20"
//	@Success	200			{object}	Envelope
//	@Router		/tasks [get]
func (s *Server) listTasks(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	page := parseInt(c.Query("page"), 1)
	pageSize := parseInt(c.Query("page_size"), 20)
	items, total, err := s.tasks.List(c.Request.Context(), repository.TaskFilter{
		ProjectID:      c.Query("project_id"),
		WorkID:         c.Query("work_id"),
		CreativeWorkID: c.Query("creative_work_id"),
		Status:         c.Query("status"),
		Type:           c.Query("type"),
		Page:           page,
		PageSize:       pageSize,
	})
	if err != nil {
		s.failFromError(c, err)
		return
	}
	resp := TaskListResponse{Items: make([]TaskResponse, 0, len(items)), Total: total, Page: page, PageSize: pageSize}
	for _, t := range items {
		resp.Items = append(resp.Items, toTaskResponse(domainTask(t)))
	}
	OK(c, resp)
}

// getTask 任务详情（进度、输出、错误）。
//
//	@Summary	任务详情
//	@Tags		tasks
//	@Produce	json
//	@Param		id	path		string	true	"任务 ID"
//	@Success	200	{object}	Envelope
//	@Failure	404	{object}	Envelope
//	@Router		/tasks/{id} [get]
func (s *Server) getTask(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	t, err := s.tasks.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		s.failFromError(c, err)
		return
	}
	OK(c, toTaskResponse(domainTask(*t)))
}

// createTask 手工入队任务（调试/扩展用）。
//
//	@Summary	手工入队任务
//	@Tags		tasks
//	@Accept		json
//	@Produce	json
//	@Param		body	body		createTaskRequest	true	"任务定义"
//	@Success	202		{object}	Envelope
//	@Failure	400		{object}	Envelope
//	@Router		/tasks [post]
func (s *Server) createTask(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	var req createTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "请求参数不合法: "+err.Error(), nil)
		return
	}
	t, err := s.tasks.Enqueue(c.Request.Context(), service.EnqueueInput{
		Type: req.Type, ProjectID: req.ProjectID, WorkID: req.WorkID, CreativeWorkID: req.CreativeWorkID,
		Input: req.Input, MaxAttempts: req.MaxAttempts,
	})
	if err != nil {
		s.failFromError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, Envelope{Data: toTaskResponse(domainTask(*t)), TraceID: traceID(c)})
}

// cancelTask 取消任务。
//
//	@Summary	取消任务
//	@Tags		tasks
//	@Produce	json
//	@Param		id	path		string	true	"任务 ID"
//	@Success	200	{object}	Envelope
//	@Failure	404	{object}	Envelope
//	@Failure	409	{object}	Envelope
//	@Router		/tasks/{id}/cancel [post]
func (s *Server) cancelTask(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	if err := s.tasks.Cancel(c.Request.Context(), c.Param("id")); err != nil {
		s.failFromError(c, err)
		return
	}
	OK(c, gin.H{"cancelled": true})
}

// retryTask 重试任务。
//
//	@Summary	重试任务
//	@Tags		tasks
//	@Produce	json
//	@Param		id	path		string	true	"任务 ID"
//	@Success	200	{object}	Envelope
//	@Failure	404	{object}	Envelope
//	@Failure	409	{object}	Envelope
//	@Router		/tasks/{id}/retry [post]
func (s *Server) retryTask(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	t, err := s.tasks.Retry(c.Request.Context(), c.Param("id"))
	if err != nil {
		s.failFromError(c, err)
		return
	}
	OK(c, toTaskResponse(domainTask(*t)))
}

// listTaskTypes 已注册的任务类型。
//
//	@Summary	任务类型清单
//	@Tags		tasks
//	@Produce	json
//	@Success	200	{object}	Envelope
//	@Router		/task-types [get]
func (s *Server) listTaskTypes(c *gin.Context) {
	types := []string{}
	if s.taskTypes != nil {
		types = s.taskTypes()
	}
	OK(c, gin.H{"items": types, "total": len(types)})
}

// reparseOriginal 重新解析原著章节（异步）。
//
//	@Summary		重新解析原著
//	@Description	用已保存的源文件重新切分章节（例如切章规则升级后重跑）。异步执行，返回 task_id 供轮询。
//	@Tags			original
//	@Produce		json
//	@Param			id	path		string	true	"原著 ID"
//	@Success		202	{object}	Envelope
//	@Failure		400	{object}	Envelope
//	@Failure		404	{object}	Envelope
//	@Router			/original/{id}/reparse [post]
func (s *Server) reparseOriginal(c *gin.Context) {
	if !s.requireServices(c) {
		return
	}
	t, err := s.tasks.EnqueueOriginalReparse(c.Request.Context(), c.Param("id"))
	if err != nil {
		s.failFromError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, Envelope{Data: toTaskResponse(domainTask(*t)), TraceID: traceID(c)})
}
