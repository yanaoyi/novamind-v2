package api

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

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
