package ai

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/yanaoyi/novamindv2/backend/internal/domain"
)

// Message 是一条对话消息。
type Message struct {
	Role    string `json:"role"` // system | user | assistant
	Content string `json:"content"`
}

// ChatRequest 是统一调用入参。
type ChatRequest struct {
	Messages    []Message
	Temperature float64
	MaxTokens   int
	// JSONMode 请求模型返回 JSON（OpenAI 兼容协议用 response_format；
	// Anthropic 靠 prompt 约束 + 后置校验）
	JSONMode   bool
	TimeoutSec int
}

// ChatResponse 是统一返回。
type ChatResponse struct {
	Content          string
	Model            string
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	LatencyMS        int64
}

// ProviderConfig 是运行时（已解密）的调用配置。
// 它只活在内存里，不落库、不进日志。
type ProviderConfig struct {
	Type      domain.ProviderType
	APIBase   string
	APIKey    string
	ModelName string
}

// APIError 是上游返回的错误，便于上层区分鉴权/限流/服务端错误。
type APIError struct {
	Provider   string
	StatusCode int
	Body       string
}

// Error 实现 error。
func (e *APIError) Error() string {
	body := e.Body
	if len(body) > 300 {
		body = body[:300] + "…"
	}
	return fmt.Sprintf("%s 接口返回 %d: %s", e.Provider, e.StatusCode, body)
}

// IsAuthError 判断是否为鉴权类错误（401/403）。
func (e *APIError) IsAuthError() bool { return e.StatusCode == 401 || e.StatusCode == 403 }

// IsRateLimited 判断是否为限流。
func (e *APIError) IsRateLimited() bool { return e.StatusCode == 429 }

// IsRetryable 判断是否值得重试（限流 / 5xx）。
func (e *APIError) IsRetryable() bool { return e.StatusCode == 429 || e.StatusCode >= 500 }

// SanitizeError 把上游错误转成可对外展示的短消息：**不回显上游响应体**。
//
// 原因：调用方（含匿名调用方）能拿到响应体就等于拿到一个内网探测 oracle ——
// 拿不同 api_base 去试，看回显内容就能判断"这个地址后面有没有服务、是什么服务"。
// 细节写服务端日志，对外只给状态码与分类文案。
func SanitizeError(err error) string {
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		return err.Error()
	}
	hint := "（上游拒绝了本次请求，详情见服务端日志）"
	switch {
	case apiErr.IsAuthError():
		hint = "（密钥或权限有问题）"
	case apiErr.IsRateLimited():
		hint = "（被上游限流）"
	case apiErr.StatusCode >= 500:
		hint = "（上游服务端错误）"
	}
	return fmt.Sprintf("%s 接口返回 %d%s", apiErr.Provider, apiErr.StatusCode, hint)
}

// Gateway 是统一模型调用入口。
type Gateway struct {
	httpClient *http.Client
	maxRetry   int
}

// NewGateway 构建网关。
func NewGateway() *Gateway {
	return &Gateway{
		httpClient: &http.Client{Timeout: 0}, // 超时按请求级别用 context 控制
		maxRetry:   3,
	}
}

// Chat 按提供商类型分发调用，内置重试（限流与 5xx）。
func (g *Gateway) Chat(ctx context.Context, cfg ProviderConfig, req ChatRequest) (ChatResponse, error) {
	if err := validateConfig(cfg); err != nil {
		return ChatResponse{}, err
	}
	if len(req.Messages) == 0 {
		return ChatResponse{}, errors.New("messages 不能为空")
	}

	timeout := time.Duration(req.TimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 120 * time.Second
	}

	var lastErr error
	for attempt := 1; attempt <= g.maxRetry; attempt++ {
		attemptCtx, cancel := context.WithTimeout(ctx, timeout)
		start := time.Now()

		var resp ChatResponse
		var err error
		switch cfg.Type {
		case domain.ProviderOpenAICompatible:
			resp, err = g.chatOpenAI(attemptCtx, cfg, req)
		case domain.ProviderAnthropic:
			resp, err = g.chatAnthropic(attemptCtx, cfg, req)
		default:
			cancel()
			return ChatResponse{}, fmt.Errorf("不支持的提供商类型: %s", cfg.Type)
		}
		cancel()

		if err == nil {
			resp.LatencyMS = time.Since(start).Milliseconds()
			return resp, nil
		}
		lastErr = err

		var apiErr *APIError
		if !errors.As(err, &apiErr) || !apiErr.IsRetryable() {
			break // 参数错/鉴权错重试没意义
		}
		if attempt < g.maxRetry {
			select {
			case <-ctx.Done():
				return ChatResponse{}, ctx.Err()
			case <-time.After(time.Duration(attempt) * 800 * time.Millisecond):
			}
		}
	}
	return ChatResponse{}, lastErr
}

func validateConfig(cfg ProviderConfig) error {
	if !cfg.Type.Valid() {
		return fmt.Errorf("提供商类型非法: %s", cfg.Type)
	}
	if strings.TrimSpace(cfg.APIBase) == "" {
		return errors.New("接口地址未配置")
	}
	if strings.TrimSpace(cfg.ModelName) == "" {
		return errors.New("模型名未配置")
	}
	return nil
}

// splitSystemMessages 把 system 消息拆出来（Anthropic 用独立字段）。
func splitSystemMessages(messages []Message) (string, []Message) {
	var system strings.Builder
	rest := make([]Message, 0, len(messages))
	for _, m := range messages {
		if strings.EqualFold(m.Role, "system") {
			if system.Len() > 0 {
				system.WriteString("\n\n")
			}
			system.WriteString(m.Content)
			continue
		}
		rest = append(rest, m)
	}
	return system.String(), rest
}
