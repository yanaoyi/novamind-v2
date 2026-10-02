package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// TraceIDKey 是贯穿请求与日志的 trace id 键名。
const TraceIDKey = "trace_id"

// Envelope 是全部 API 的统一响应包（SPEC.md §26 / ARCHITECTURE §5）。
type Envelope struct {
	Data    any       `json:"data"`
	Error   *APIError `json:"error"`
	TraceID string    `json:"trace_id"`
}

// APIError 是统一错误体。
type APIError struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

// 错误码常量：客户端按 Code 判断，不解析 Message。
const (
	CodeBadRequest = "BAD_REQUEST"
	CodeNotFound   = "NOT_FOUND"
	CodeInternal   = "INTERNAL_ERROR"
	CodeConflict   = "CONFLICT"
)

// OK 返回成功响应。
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Envelope{Data: data, TraceID: traceID(c)})
}

// Created 返回 201。
func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, Envelope{Data: data, TraceID: traceID(c)})
}

// Fail 返回失败响应。
func Fail(c *gin.Context, status int, code, message string, details map[string]any) {
	c.AbortWithStatusJSON(status, Envelope{
		Error:   &APIError{Code: code, Message: message, Details: details},
		TraceID: traceID(c),
	})
}

func traceID(c *gin.Context) string {
	if v, ok := c.Get(TraceIDKey); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
