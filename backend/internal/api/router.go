package api

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/yanaoyi/novamindv2/backend/internal/ai"
	"github.com/yanaoyi/novamindv2/backend/internal/config"
	"github.com/yanaoyi/novamindv2/backend/internal/service"
)

// Server 持有 HTTP 层依赖。
// 约束：api 层只做参数校验与响应组装，业务逻辑放在 service。
type Server struct {
	cfg        *config.Config
	logger     *slog.Logger
	deps       HealthDeps
	projects   *service.ProjectService
	originals  *service.OriginalService
	characters *service.OriginalCharacterService
	worlds     *service.OriginalWorldService
	events     *service.OriginalEventService
	providers  *service.ModelProviderService
	prompts    *ai.Engine
	tasks      *service.TaskService
	taskTypes  func() []string
	analysis   *service.AnalysisService
	creative   *service.CreativeService
	writing    *service.WritingService
	versions   *service.VersionService
	invoker    *service.ModelInvoker
}

// NewServer 构建 HTTP 服务。
func NewServer(
	cfg *config.Config,
	logger *slog.Logger,
	deps HealthDeps,
	projects *service.ProjectService,
	originals *service.OriginalService,
	characters *service.OriginalCharacterService,
	worlds *service.OriginalWorldService,
	events *service.OriginalEventService,
	providers *service.ModelProviderService,
	promptEngine *ai.Engine,
	tasks *service.TaskService,
	taskTypes func() []string,
	analysis *service.AnalysisService,
	creative *service.CreativeService,
	writing *service.WritingService,
	versions *service.VersionService,
	invoker *service.ModelInvoker,
) *Server {
	return &Server{
		cfg: cfg, logger: logger, deps: deps,
		projects: projects, originals: originals, characters: characters, worlds: worlds,
		events: events, providers: providers, prompts: promptEngine,
		tasks: tasks, taskTypes: taskTypes, analysis: analysis, creative: creative,
		writing: writing, invoker: invoker,
		versions: versions,
	}
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

		// 工程管理（SPEC.md §26 Projects）
		projects := v1.Group("/projects")
		{
			projects.GET("", s.listProjects)
			projects.POST("", s.createProject)
			projects.GET("/:id", s.getProject)
			projects.PUT("/:id", s.updateProject)
			projects.DELETE("/:id", s.deleteProject)
			projects.POST("/:id/original", s.createOriginal)
		}

		// 原著（SPEC.md §26 Original）
		original := v1.Group("/original")
		{
			original.GET("/:id", s.getOriginal)
			original.POST("/:id/import", s.importOriginal)
			original.GET("/:id/chapters", s.listOriginalChapters)
			original.GET("/:id/chapters/:no", s.getOriginalChapter)
			original.GET("/:id/characters", s.listCharacters)
			original.POST("/:id/characters", s.createCharacter)
			original.GET("/:id/relationships", s.listRelationships)
			original.POST("/:id/relationships", s.createRelationship)
			original.GET("/:id/world", s.getWorld)
			original.PUT("/:id/world", s.upsertWorld)
			original.GET("/:id/rules", s.listWorldRules)
			original.POST("/:id/rules", s.createWorldRule)
			original.GET("/:id/locations", s.listLocations)
			original.POST("/:id/locations", s.createLocation)
			original.GET("/:id/factions", s.listFactions)
			original.POST("/:id/factions", s.createFaction)
			original.GET("/:id/events", s.listEvents)
			original.POST("/:id/events", s.createEvent)
			original.GET("/:id/timeline", s.getTimeline)
			original.PUT("/:id/timeline", s.setTimelineOrder)
			original.GET("/:id/plot-arcs", s.listPlotArcs)
			original.POST("/:id/plot-arcs", s.createPlotArc)
			original.POST("/:id/reparse", s.reparseOriginal)
			original.POST("/:id/analysis", s.enqueueAnalysis)
			original.GET("/:id/proposals", s.listProposals)
			original.GET("/:id/analysis/summary", s.analysisSummary)
			// 二创（SPEC.md §9.1-§9.5）
			original.POST("/:id/create-creative", s.createCreativeWork)
			original.GET("/:id/creative-works", s.listCreativeWorks)
		}

		creative := v1.Group("/creative")
		{
			creative.GET("/:id", s.getCreativeWork)
			creative.PUT("/:id", s.updateCreativeWork)
			creative.GET("/:id/characters", s.listCreativeCharacters)
			creative.POST("/:id/characters/inherit", s.inheritCreativeCharacter)
			creative.POST("/:id/characters/new", s.createNewCreativeCharacter)
			creative.POST("/:id/characters/fuse", s.fuseCreativeCharacters)
			creative.GET("/:id/mappings", s.listCreativeMappings)
			// 二创世界 / 分叉点 / 二创时间线（SPEC.md §10.1-§10.4）
			creative.GET("/:id/world", s.getCreativeWorld)
			creative.PUT("/:id/world", s.updateCreativeWorld)
			creative.POST("/:id/world/inherit", s.inheritCreativeWorld)
			creative.POST("/:id/world/rules", s.createCreativeWorldRule)
			creative.GET("/:id/divergence", s.getDivergence)
			creative.PUT("/:id/divergence", s.setDivergence)
			creative.GET("/:id/timeline", s.getCreativeTimeline)
			creative.PUT("/:id/timeline", s.setCreativeTimeline)
			creative.POST("/:id/timeline/build", s.buildCreativeTimeline)
			// 写作系统（SPEC.md §12.1-§12.3、§12.5、§14、§30）
			creative.POST("/:id/volumes", s.createVolume)
			creative.GET("/:id/volumes", s.listVolumes)
			creative.POST("/:id/chapters", s.createChapter)
			creative.GET("/:id/chapters", s.listCreativeChapters)
			creative.POST("/:id/consistency/check", s.checkConsistency)
			creative.GET("/:id/consistency/issues", s.listConsistencyIssues)
			creative.GET("/:id/export", s.exportCreative)
			// 版本历史（SPEC.md §13）：世界观 / 大纲
			creative.GET("/:id/world/versions", s.listWorldVersions)
			creative.POST("/:id/world/versions", s.snapshotWorldHandler)
			creative.GET("/:id/world/versions/:no", s.getWorldVersion)
			creative.POST("/:id/world/versions/:no/restore", s.restoreWorldVersion)
			creative.GET("/:id/outline/versions", s.listOutlineVersions)
			creative.POST("/:id/outline/versions", s.snapshotOutlineHandler)
			creative.GET("/:id/outline/versions/:no", s.getOutlineVersion)
			creative.POST("/:id/outline/versions/:no/restore", s.restoreOutlineVersion)
		}
		chapters := v1.Group("/chapters")
		{
			chapters.GET("/:id", s.getCreativeChapter)
			chapters.PUT("/:id", s.updateCreativeChapter)
			chapters.DELETE("/:id", s.deleteCreativeChapter)
			chapters.POST("/:id/generate", s.generateChapter)
			chapters.GET("/:id/versions", s.listChapterVersions)
			chapters.GET("/:id/versions/:no", s.getChapterVersion)
			chapters.POST("/:id/versions/:no/restore", s.restoreChapterVersion)
			chapters.POST("/:id/scenes", s.createChapterScene)
			chapters.GET("/:id/scenes", s.listChapterScenes)
		}
		v1.POST("/ai/rewrite", s.rewriteText)
		consistencyIssues := v1.Group("/consistency-issues")
		{
			consistencyIssues.PUT("/:id", s.updateConsistencyIssue)
		}
		creativeWorldRules := v1.Group("/creative-world-rules")
		{
			creativeWorldRules.PUT("/:id", s.updateCreativeWorldRule)
			creativeWorldRules.DELETE("/:id", s.deleteCreativeWorldRule)
		}
		creativeCharacters := v1.Group("/creative-characters")
		{
			creativeCharacters.GET("/:id", s.getCreativeCharacter)
			creativeCharacters.PUT("/:id", s.updateCreativeCharacter)
			creativeCharacters.DELETE("/:id", s.deleteCreativeCharacter)
			creativeCharacters.GET("/:id/versions", s.listCharacterVersions)
			creativeCharacters.POST("/:id/versions", s.snapshotCharacterHandler)
			creativeCharacters.GET("/:id/versions/:no", s.getCharacterVersion)
			creativeCharacters.POST("/:id/versions/:no/restore", s.restoreCharacterVersion)
		}
		mappings := v1.Group("/mappings")
		{
			mappings.DELETE("/:id", s.deleteCreativeMapping)
		}

		// AI 提案审核（SPEC.md §22.2：AI 结果必须经作者确认）
		proposals := v1.Group("/proposals")
		{
			proposals.GET("/:id", s.getProposal)
			proposals.POST("/:id/approve", s.approveProposal)
			proposals.POST("/:id/reject", s.rejectProposal)
		}

		// 事件 / 剧情弧（SPEC.md §8.1、§8.3）
		events := v1.Group("/events")
		{
			events.GET("/:id", s.getEvent)
			events.PUT("/:id", s.updateEvent)
			events.DELETE("/:id", s.deleteEvent)
		}
		plotArcs := v1.Group("/plot-arcs")
		{
			plotArcs.PUT("/:id", s.updatePlotArc)
			plotArcs.DELETE("/:id", s.deletePlotArc)
		}

		// AI 接入配置（SPEC.md §16 Model Gateway）与 Prompt 清单（§17）
		modelProviders := v1.Group("/model-providers")
		{
			modelProviders.GET("", s.listModelProviders)
			modelProviders.POST("", s.createModelProvider)
			modelProviders.GET("/:id", s.getModelProvider)
			modelProviders.PUT("/:id", s.updateModelProvider)
			modelProviders.DELETE("/:id", s.deleteModelProvider)
			modelProviders.POST("/:id/default", s.setDefaultModelProvider)
			modelProviders.POST("/:id/test", s.testModelProvider)
		}
		v1.GET("/prompts", s.listPrompts)

		// 异步任务（SPEC.md §27）
		tasks := v1.Group("/tasks")
		{
			tasks.GET("", s.listTasks)
			tasks.POST("", s.createTask)
			tasks.GET("/:id", s.getTask)
			tasks.POST("/:id/cancel", s.cancelTask)
			tasks.POST("/:id/retry", s.retryTask)
		}
		v1.GET("/task-types", s.listTaskTypes)

		// 人物与关系（SPEC.md §6.1-§6.3）
		characters := v1.Group("/characters")
		{
			characters.GET("/:id", s.getCharacter)
			characters.PUT("/:id", s.updateCharacter)
			characters.DELETE("/:id", s.deleteCharacter)
		}
		relationships := v1.Group("/relationships")
		{
			relationships.PUT("/:id", s.updateRelationship)
			relationships.DELETE("/:id", s.deleteRelationship)
		}

		// 世界观：规则 / 地点 / 势力（SPEC.md §7）
		worldRules := v1.Group("/world-rules")
		{
			worldRules.PUT("/:id", s.updateWorldRule)
			worldRules.DELETE("/:id", s.deleteWorldRule)
		}
		locations := v1.Group("/locations")
		{
			locations.PUT("/:id", s.updateLocation)
			locations.DELETE("/:id", s.deleteLocation)
		}
		factions := v1.Group("/factions")
		{
			factions.PUT("/:id", s.updateFaction)
			factions.DELETE("/:id", s.deleteFaction)
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
	if s.projects == nil || s.originals == nil || s.characters == nil || s.worlds == nil ||
		s.events == nil || s.providers == nil || s.tasks == nil || s.analysis == nil ||
		s.creative == nil || s.writing == nil || s.versions == nil {
		Fail(c, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE",
			"服务未就绪：数据库未连接或初始化失败", nil)
		return false
	}
	return true
}
