package api

import (
	"crypto/subtle"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Auth 校验访问令牌（Bearer）。token 为空表示未配置（开发环境），此时放行并靠启动日志告警；
// 生产环境未配置令牌会在 config.Load 阶段直接拒绝启动，不会走到这里。
//
// 放行清单只含"不含业务数据"的端点：健康检查、OpenAPI 文档、Swagger UI。
func Auth(token string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if token == "" || isAuthExempt(c) {
			c.Next()
			return
		}
		got := bearerToken(c.GetHeader("Authorization"))
		if got == "" || subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
			Fail(c, http.StatusUnauthorized, CodeUnauthorized,
				"需要访问令牌：请在请求头带上 Authorization: Bearer <ADMIN_TOKEN>", nil)
			c.Abort()
			return
		}
		c.Next()
	}
}

func isAuthExempt(c *gin.Context) bool {
	path := c.Request.URL.Path
	if path == "/api/v1/health" || path == "/api/v1/openapi.yaml" {
		return true
	}
	return strings.HasPrefix(path, "/swagger")
}

func bearerToken(header string) string {
	const prefix = "Bearer "
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return ""
	}
	return strings.TrimSpace(header[len(prefix):])
}

// rateLimiter 是"固定窗口 + 按客户端 IP 计数"的简易限流器：
// 给"会真花钱"的端点（/ai/*、模型连通性测试）装一道闸门，避免 Key 额度被刷光。
// 进程内计数，重启归零，够用且不引入新依赖。
type rateLimiter struct {
	mu      sync.Mutex
	hits    map[string]int
	limit   int
	window  time.Duration
	resetAt time.Time
}

// RateLimit 返回限流中间件：window 时间内同一 IP 最多 limit 次。
func RateLimit(limit int, window time.Duration) gin.HandlerFunc {
	limiter := &rateLimiter{hits: map[string]int{}, limit: limit, window: window, resetAt: time.Now().Add(window)}
	return func(c *gin.Context) {
		if limit <= 0 {
			c.Next()
			return
		}
		if !limiter.allow(c.ClientIP()) {
			Fail(c, http.StatusTooManyRequests, CodeTooManyRequests,
				"请求过于频繁，请稍后再试",
				map[string]any{"limit": limit, "window_seconds": int(window.Seconds())})
			c.Abort()
			return
		}
		c.Next()
	}
}

func (r *rateLimiter) allow(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	if now.After(r.resetAt) {
		r.hits = map[string]int{}
		r.resetAt = now.Add(r.window)
	}
	r.hits[key]++
	return r.hits[key] <= r.limit
}

// RequestIDHeader 是外部传入 trace id 的请求头。
const RequestIDHeader = "X-Request-Id"

// RequestID 为每个请求分配 trace id（优先沿用调用方传入的值）。
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(RequestIDHeader)
		if id == "" {
			id = uuid.NewString()
		}
		c.Set(TraceIDKey, id)
		c.Header(RequestIDHeader, id)
		c.Next()
	}
}

// Logger 结构化记录每个请求（含 trace_id、状态码、耗时）。
func Logger(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		if logger == nil {
			return
		}
		logger.Info("http_request",
			slog.String("trace_id", traceID(c)),
			slog.String("method", c.Request.Method),
			slog.String("path", c.Request.URL.Path),
			slog.Int("status", c.Writer.Status()),
			slog.Duration("latency", time.Since(start)),
		)
	}
}

// Recovery 把 panic 转成统一的 500 响应，避免把堆栈泄露给客户端。
func Recovery(logger *slog.Logger) gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered any) {
		if logger != nil {
			logger.Error("panic_recovered",
				slog.String("trace_id", traceID(c)),
				slog.String("path", c.Request.URL.Path),
				slog.Any("error", recovered),
			)
		}
		Fail(c, 500, CodeInternal, "服务器内部错误", nil)
	})
}
