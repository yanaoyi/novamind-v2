package api

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/yanaoyi/novamindv2/backend/internal/config"
	"github.com/yanaoyi/novamindv2/backend/internal/service"
)

// Server 持有 HTTP 层依赖。
// 约束：api 层只做参数校验与响应组装，业务逻辑放在 service。
type Server struct {
	cfg       *config.Config
	logger    *slog.Logger
	deps      HealthDeps
	projects  *service.ProjectService
	originals *service.OriginalService
}

// NewServer 构建 HTTP 服务。
func NewServer(
	cfg *config.Config,
	logger *slog.Logger,
	deps HealthDeps,
	projects *service.ProjectService,
	originals *service.OriginalService,
) *Server {
	return &Server{cfg: cfg, logger: logger, deps: deps, projects: projects, originals: originals}
}

// Router 组装路由与中间件。
//
// 注意：所有业务路由**恒定注册**，不因某个 service 未初始化而消失——
// 否则"数据库没连上"会表现为 404（接口不存在），把配置问题伪装成路由问题。
// 服务未就绪时由处理函数返回 503，语义明确。
func (s *Server) Router() *gin.Engine {
	if s.cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(RequestID(), Logger(s.logger), Recovery(s.logger))

	v1 := r.Group("/api/v1")
	{
		v1.GET("/health", s.handleHealth)
		v1.GET("/openapi.yaml", s.handleOpenAPISpec)

		// 工程管理（规格书 §49 Projects）
		projects := v1.Group("/projects")
		{
			projects.GET("", s.listProjects)
			projects.POST("", s.createProject)
			projects.GET("/:id", s.getProject)
			projects.PUT("/:id", s.updateProject)
			projects.DELETE("/:id", s.deleteProject)
			projects.POST("/:id/original", s.createOriginal)
		}

		// 原著（规格书 §49 Original）
		original := v1.Group("/original")
		{
			original.GET("/:id", s.getOriginal)
			original.POST("/:id/import", s.importOriginal)
			original.GET("/:id/chapters", s.listOriginalChapters)
			original.GET("/:id/chapters/:no", s.getOriginalChapter)
		}
	}

	r.NoRoute(func(c *gin.Context) {
		Fail(c, 404, CodeNotFound, "接口不存在", map[string]any{"path": c.Request.URL.Path})
	})

	registerSwagger(r)
	return r
}

// requireServices 确认业务服务已就绪（数据库已连接）。
// 未就绪时返回 503，而不是让请求打到 nil 上 panic 或静默 404。
func (s *Server) requireServices(c *gin.Context) bool {
	if s.projects == nil || s.originals == nil {
		Fail(c, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE",
			"服务未就绪：数据库未连接或初始化失败", nil)
		return false
	}
	return true
}
