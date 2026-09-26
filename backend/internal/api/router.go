package api

import (
	"log/slog"

	"github.com/gin-gonic/gin"

	"github.com/yanaoyi/novamindv2/backend/internal/config"
	"github.com/yanaoyi/novamindv2/backend/internal/service"
)

// Server 持有 HTTP 层依赖。
// 约束：api 层只做参数校验与响应组装，业务逻辑放在 service。
type Server struct {
	cfg      *config.Config
	logger   *slog.Logger
	deps     HealthDeps
	projects *service.ProjectService
}

// NewServer 构建 HTTP 服务。
func NewServer(cfg *config.Config, logger *slog.Logger, deps HealthDeps, projects *service.ProjectService) *Server {
	return &Server{cfg: cfg, logger: logger, deps: deps, projects: projects}
}

// Router 组装路由与中间件。
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
		if s.projects != nil {
			projects := v1.Group("/projects")
			{
				projects.GET("", s.listProjects)
				projects.POST("", s.createProject)
				projects.GET("/:id", s.getProject)
				projects.PUT("/:id", s.updateProject)
				projects.DELETE("/:id", s.deleteProject)
			}
		}
	}

	r.NoRoute(func(c *gin.Context) {
		Fail(c, 404, CodeNotFound, "接口不存在", map[string]any{"path": c.Request.URL.Path})
	})

	registerSwagger(r)
	return r
}
