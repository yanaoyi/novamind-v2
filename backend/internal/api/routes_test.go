package api

import (
	"log/slog"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"

	"github.com/yanaoyi/novamindv2/backend/internal/config"
)

// TestOpenAPICoversAllRoutes 是防漂移测试：
// 任何已注册的 API 路由都必须在 openapi.yaml 中有对应定义，否则失败。
// 这样"代码加了接口但忘了写文档"会立刻被测试抓住。
func TestOpenAPICoversAllRoutes(t *testing.T) {
	paths := loadSpecPaths(t)

	// 传 nil 服务：路由表恒定注册，本测试只校验路由与规范一致，不调用处理函数
	srv := NewServer(&config.Config{}, slog.Default(), HealthDeps{},
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	router := srv.Router()

	for _, problem := range verifyRouteCoverage(router.Routes(), paths) {
		t.Error(problem)
	}
}

// TestVerifyRouteCoverageDetectsMissingRoute 反向用例：
// 证明校验逻辑真的能发现"代码有接口、规范里没有"的情况（否则这个护栏形同虚设）。
func TestVerifyRouteCoverageDetectsMissingRoute(t *testing.T) {
	paths := map[string]map[string]any{
		"/projects": {"get": nil},
	}
	routes := []gin.RouteInfo{
		{Method: "GET", Path: "/api/v1/projects"},     // 已覆盖
		{Method: "POST", Path: "/api/v1/projects"},    // 规范里没有 post
		{Method: "GET", Path: "/api/v1/original/:id"}, // 规范里整个 path 都没有
		{Method: "GET", Path: "/swagger/*any"},        // 应被忽略
	}

	problems := verifyRouteCoverage(routes, paths)
	if len(problems) != 2 {
		t.Fatalf("应报告 2 个问题，实际 %d 个: %v", len(problems), problems)
	}
	joined := strings.Join(problems, " | ")
	if !strings.Contains(joined, "POST /api/v1/projects") {
		t.Errorf("未检出缺失的 post 操作: %s", joined)
	}
	if !strings.Contains(joined, "/api/v1/original/:id") {
		t.Errorf("未检出完全缺失的 path: %s", joined)
	}
}

func loadSpecPaths(t *testing.T) map[string]map[string]any {
	t.Helper()
	var spec struct {
		Paths map[string]map[string]any `yaml:"paths"`
	}
	if err := yaml.Unmarshal(OpenAPISpec(), &spec); err != nil {
		t.Fatalf("openapi.yaml 解析失败: %v", err)
	}
	if len(spec.Paths) == 0 {
		t.Fatal("openapi.yaml 中没有任何 path，规范文件可能损坏")
	}
	return spec.Paths
}

// verifyRouteCoverage 返回所有"未在规范中定义"的路由问题描述。
func verifyRouteCoverage(routes []gin.RouteInfo, paths map[string]map[string]any) []string {
	var problems []string
	for _, route := range routes {
		if strings.HasPrefix(route.Path, "/swagger") {
			continue // Swagger UI 自身不属于业务 API
		}
		apiPath := strings.TrimPrefix(route.Path, "/api/v1")
		if apiPath == "" {
			apiPath = "/"
		}
		apiPath = convertGinPath(apiPath)

		operations, ok := paths[apiPath]
		if !ok {
			problems = append(problems, route.Method+" "+route.Path+" 未在 openapi.yaml 的 paths 中定义")
			continue
		}
		if _, ok := operations[strings.ToLower(route.Method)]; !ok {
			problems = append(problems, route.Method+" "+route.Path+"（规范路径 "+apiPath+"）缺少 "+
				strings.ToLower(route.Method)+" 操作定义")
		}
	}
	return problems
}

// TestOpenAPISpecHasServersAndInfo 做最低限度的结构校验。
func TestOpenAPISpecHasServersAndInfo(t *testing.T) {
	content := string(OpenAPISpec())
	for _, must := range []string{"openapi: 3.0.3", "title: NovaMind API", "url: /api/v1", "components:"} {
		if !strings.Contains(content, must) {
			t.Errorf("openapi.yaml 缺少必需内容: %q", must)
		}
	}
}

// convertGinPath 把 gin 的 :param 语法转成 OpenAPI 的 {param} 语法。
func convertGinPath(path string) string {
	segments := strings.Split(path, "/")
	for i, seg := range segments {
		if strings.HasPrefix(seg, ":") {
			segments[i] = "{" + strings.TrimPrefix(seg, ":") + "}"
		}
	}
	return strings.Join(segments, "/")
}
