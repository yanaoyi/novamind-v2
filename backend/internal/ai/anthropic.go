package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/yanaoyi/novamindv2/backend/internal/domain"
)

// anthropicVersion 是 Anthropic Messages API 的版本头。
const anthropicVersion = "2023-06-01"

type anthropicRequest struct {
	Model       string    `json:"model"`
	System      string    `json:"system,omitempty"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens"`
	Temperature float64   `json:"temperature,omitempty"`
}

type anthropicResponse struct {
	Model   string `json:"model"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// chatAnthropic 调用 Anthropic Messages API（Claude）。
func (g *Gateway) chatAnthropic(ctx context.Context, cfg ProviderConfig, req ChatRequest) (ChatResponse, error) {
	system, messages := splitSystemMessages(req.Messages)
	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4096 // Anthropic 的 max_tokens 必填
	}

	body := anthropicRequest{
		Model:       cfg.ModelName,
		System:      system,
		Messages:    messages,
		MaxTokens:   maxTokens,
		Temperature: req.Temperature,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return ChatResponse{}, fmt.Errorf("构造请求失败: %w", err)
	}

	base := strings.TrimRight(cfg.APIBase, "/")
	// 允许用户填 https://api.anthropic.com 或带 /v1
	endpoint := base + "/v1/messages"
	if strings.HasSuffix(base, "/v1") {
		endpoint = base + "/messages"
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return ChatResponse{}, fmt.Errorf("构造 HTTP 请求失败: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("anthropic-version", anthropicVersion)
	if cfg.APIKey != "" {
		httpReq.Header.Set("x-api-key", cfg.APIKey)
	}

	resp, err := g.httpClient.Do(httpReq)
	if err != nil {
		return ChatResponse{}, fmt.Errorf("调用模型接口失败: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return ChatResponse{}, fmt.Errorf("读取响应失败: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return ChatResponse{}, &APIError{
			Provider: string(domain.ProviderAnthropic), StatusCode: resp.StatusCode, Body: string(raw),
		}
	}

	var parsed anthropicResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return ChatResponse{}, fmt.Errorf("解析响应失败: %w（原始内容前 200 字：%s）", err, head(string(raw), 200))
	}
	if parsed.Error != nil {
		return ChatResponse{}, fmt.Errorf("模型返回错误: %s", parsed.Error.Message)
	}
	var sb strings.Builder
	for _, block := range parsed.Content {
		if block.Type == "text" {
			sb.WriteString(block.Text)
		}
	}
	if sb.Len() == 0 {
		return ChatResponse{}, fmt.Errorf("模型没有返回文本内容（原始内容前 200 字：%s）", head(string(raw), 200))
	}

	return ChatResponse{
		Content:          sb.String(),
		Model:            parsed.Model,
		PromptTokens:     parsed.Usage.InputTokens,
		CompletionTokens: parsed.Usage.OutputTokens,
		TotalTokens:      parsed.Usage.InputTokens + parsed.Usage.OutputTokens,
	}, nil
}
