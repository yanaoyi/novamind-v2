package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func newAuthRouter(token string, rateLimit int) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestID(), Auth(token))
	r.GET("/api/v1/health", func(c *gin.Context) { OK(c, gin.H{"status": "ok"}) })
	r.GET("/api/v1/projects", func(c *gin.Context) { OK(c, gin.H{"items": []any{}}) })
	if rateLimit > 0 {
		r.POST("/api/v1/ai/chat", RateLimit(rateLimit, time.Minute), func(c *gin.Context) { OK(c, gin.H{"reply": "好"}) })
	}
	return r
}

func do(r *gin.Engine, method, path, authHeader string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestAuthRequiresBearerToken(t *testing.T) {
	r := newAuthRouter("s3cret-token", 0)

	if w := do(r, http.MethodGet, "/api/v1/projects", ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("无令牌应 401，实际 %d", w.Code)
	}
	if w := do(r, http.MethodGet, "/api/v1/projects", "Bearer wrong"); w.Code != http.StatusUnauthorized {
		t.Fatalf("错误令牌应 401，实际 %d", w.Code)
	}
	if w := do(r, http.MethodGet, "/api/v1/projects", "s3cret-token"); w.Code != http.StatusUnauthorized {
		t.Fatalf("缺少 Bearer 前缀应 401，实际 %d", w.Code)
	}
	if w := do(r, http.MethodGet, "/api/v1/projects", "Bearer s3cret-token"); w.Code != http.StatusOK {
		t.Fatalf("正确令牌应 200，实际 %d", w.Code)
	}

	// 401 的响应体必须是统一包 + 明确错误码，前端能据此提示"填令牌"
	w := do(r, http.MethodGet, "/api/v1/projects", "")
	var env Envelope
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("响应不是统一包: %v", err)
	}
	if env.Error == nil || env.Error.Code != CodeUnauthorized {
		t.Fatalf("错误码应为 %s，实际 %+v", CodeUnauthorized, env.Error)
	}
}

func TestAuthExemptsHealthAndDocs(t *testing.T) {
	r := newAuthRouter("s3cret-token", 0)
	if w := do(r, http.MethodGet, "/api/v1/health", ""); w.Code != http.StatusOK {
		t.Fatalf("健康检查应放行，实际 %d", w.Code)
	}
}

func TestAuthDisabledWhenTokenEmpty(t *testing.T) {
	r := newAuthRouter("", 0)
	if w := do(r, http.MethodGet, "/api/v1/projects", ""); w.Code != http.StatusOK {
		t.Fatalf("未配置令牌时放行（开发环境），实际 %d", w.Code)
	}
}

func TestRateLimitReturns429(t *testing.T) {
	r := newAuthRouter("", 2)
	for i := 1; i <= 2; i++ {
		if w := do(r, http.MethodPost, "/api/v1/ai/chat", ""); w.Code != http.StatusOK {
			t.Fatalf("第 %d 次应放行，实际 %d", i, w.Code)
		}
	}
	w := do(r, http.MethodPost, "/api/v1/ai/chat", "")
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("超过阈值应 429，实际 %d", w.Code)
	}
	var env Envelope
	_ = json.Unmarshal(w.Body.Bytes(), &env)
	if env.Error == nil || env.Error.Code != CodeTooManyRequests {
		t.Fatalf("429 错误码应为 %s，实际 %+v", CodeTooManyRequests, env.Error)
	}
}
