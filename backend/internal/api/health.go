package api

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// DependencyStatus 是单个依赖的健康状态。
type DependencyStatus struct {
	Status string `json:"status"` // ok | error | not_configured
	Detail string `json:"detail,omitempty"`
}

// HealthReport 是 /api/v1/health 的响应体。
type HealthReport struct {
	Status    string                      `json:"status"` // ok | degraded
	Version   string                      `json:"version"`
	Env       string                      `json:"env"`
	UptimeSec int64                       `json:"uptime_sec"`
	Checks    map[string]DependencyStatus `json:"checks"`
}

// Checker 是依赖的健康检查函数；返回 nil 表示健康。
type Checker func(ctx context.Context) error

// HealthDeps 是健康检查所需的下游依赖。
// P1-3 会注入真实的 PostgreSQL / Redis 检查函数；
// 未注入时状态为 not_configured，避免健康检查"假装健康"。
type HealthDeps struct {
	Version  string
	Env      string
	Started  time.Time
	Postgres Checker
	Redis    Checker
}

func (s *Server) handleHealth(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	checks := map[string]DependencyStatus{}
	healthy := true
	for name, checker := range map[string]Checker{"postgres": s.deps.Postgres, "redis": s.deps.Redis} {
		checks[name] = runChecker(ctx, checker)
		if checks[name].Status == "error" {
			healthy = false
		}
	}

	status := "ok"
	if !healthy {
		status = "degraded"
	}
	report := HealthReport{
		Status:    status,
		Version:   s.deps.Version,
		Env:       s.deps.Env,
		UptimeSec: int64(time.Since(s.deps.Started).Seconds()),
		Checks:    checks,
	}

	code := http.StatusOK
	if !healthy {
		code = http.StatusServiceUnavailable
	}
	c.JSON(code, Envelope{Data: report, TraceID: traceID(c)})
}

func runChecker(ctx context.Context, checker Checker) DependencyStatus {
	if checker == nil {
		return DependencyStatus{Status: "not_configured"}
	}
	if err := checker(ctx); err != nil {
		return DependencyStatus{Status: "error", Detail: err.Error()}
	}
	return DependencyStatus{Status: "ok"}
}
