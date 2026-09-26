package api

import (
	_ "embed"
	"net/http"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// OpenAPIPath 是规范文件的对外地址，Swagger UI 与测试都引用它。
const OpenAPIPath = "/api/v1/openapi.yaml"

//go:embed openapi.yaml
var openAPISpec []byte

// OpenAPISpec 返回内嵌的 OpenAPI 规范（供测试校验路由覆盖度）。
func OpenAPISpec() []byte { return openAPISpec }

func (s *Server) handleOpenAPISpec(c *gin.Context) {
	c.Data(http.StatusOK, "application/yaml; charset=utf-8", openAPISpec)
}

// registerSwagger 挂载 Swagger UI（静态资源内嵌，不依赖 CDN）。
func registerSwagger(r *gin.Engine) {
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler, ginSwagger.URL(OpenAPIPath)))
}
