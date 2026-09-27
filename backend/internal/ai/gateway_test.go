package ai

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yanaoyi/novamindv2/backend/internal/domain"
)

func TestChatOpenAICompatible(t *testing.T) {
	var captured map[string]any
	var authHeader string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("路径应为 /chat/completions，实际 %s", r.URL.Path)
		}
		authHeader = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &captured)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"model": "deepseek-chat",
			"choices": [{"message": {"content": "{\"ok\":true}"}, "finish_reason": "stop"}],
			"usage": {"prompt_tokens": 11, "completion_tokens": 7, "total_tokens": 18}
		}`))
	}))
	defer server.Close()

	gateway := NewGateway()
	resp, err := gateway.Chat(context.Background(), ProviderConfig{
		Type: domain.ProviderOpenAICompatible, APIBase: server.URL, APIKey: "sk-test", ModelName: "deepseek-chat",
	}, ChatRequest{
		Messages:    []Message{{Role: "user", Content: "你好"}},
		Temperature: 0.3,
		MaxTokens:   128,
		JSONMode:    true,
		TimeoutSec:  10,
	})
	if err != nil {
		t.Fatalf("调用失败: %v", err)
	}

	if resp.Content != `{"ok":true}` || resp.TotalTokens != 18 || resp.Model != "deepseek-chat" {
		t.Fatalf("响应解析不正确: %+v", resp)
	}
	if authHeader != "Bearer sk-test" {
		t.Errorf("Authorization 头不正确: %q", authHeader)
	}
	if captured["model"] != "deepseek-chat" {
		t.Errorf("model 字段不正确: %v", captured["model"])
	}
	if rf, ok := captured["response_format"].(map[string]any); !ok || rf["type"] != "json_object" {
		t.Errorf("JSONMode 应带上 response_format=json_object，实际 %v", captured["response_format"])
	}
	if captured["stream"] != false {
		t.Errorf("stream 应为 false，实际 %v", captured["stream"])
	}
}

func TestChatOpenAIRetriesOnServerError(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":{"message":"boom"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"model":"m","choices":[{"message":{"content":"第二次成功"}}],"usage":{}}`))
	}))
	defer server.Close()

	gateway := NewGateway()
	resp, err := gateway.Chat(context.Background(), ProviderConfig{
		Type: domain.ProviderOpenAICompatible, APIBase: server.URL, APIKey: "k", ModelName: "m",
	}, ChatRequest{Messages: []Message{{Role: "user", Content: "hi"}}, TimeoutSec: 10})
	if err != nil {
		t.Fatalf("5xx 后重试应成功: %v", err)
	}
	if resp.Content != "第二次成功" {
		t.Fatalf("内容不正确: %q", resp.Content)
	}
	if atomic.LoadInt32(&calls) != 2 {
		t.Errorf("应恰好调用 2 次，实际 %d", calls)
	}
}

func TestChatOpenAIAuthErrorIsNotRetried(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid api key"}}`))
	}))
	defer server.Close()

	gateway := NewGateway()
	_, err := gateway.Chat(context.Background(), ProviderConfig{
		Type: domain.ProviderOpenAICompatible, APIBase: server.URL, APIKey: "bad", ModelName: "m",
	}, ChatRequest{Messages: []Message{{Role: "user", Content: "hi"}}, TimeoutSec: 10})
	if err == nil {
		t.Fatal("401 应返回错误")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) || !apiErr.IsAuthError() {
		t.Fatalf("应是 APIError 且判定为鉴权错误，实际 %v", err)
	}
	if atomic.LoadInt32(&calls) != 1 {
		t.Errorf("鉴权错误不应重试，实际调用 %d 次", calls)
	}
}

func TestChatOpenAIMalformedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`这不是 JSON`))
	}))
	defer server.Close()

	gateway := NewGateway()
	_, err := gateway.Chat(context.Background(), ProviderConfig{
		Type: domain.ProviderOpenAICompatible, APIBase: server.URL, APIKey: "k", ModelName: "m",
	}, ChatRequest{Messages: []Message{{Role: "user", Content: "hi"}}, TimeoutSec: 10})
	if err == nil || !strings.Contains(err.Error(), "解析响应失败") {
		t.Fatalf("应提示解析失败并带上原始内容，实际 %v", err)
	}
}

func TestChatAnthropicSplitsSystemAndSetsHeaders(t *testing.T) {
	var captured map[string]any
	var keyHeader, versionHeader string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Errorf("路径应为 /v1/messages，实际 %s", r.URL.Path)
		}
		keyHeader = r.Header.Get("x-api-key")
		versionHeader = r.Header.Get("anthropic-version")
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &captured)

		_, _ = w.Write([]byte(`{
			"model": "claude-3-5-sonnet",
			"content": [{"type": "text", "text": "收到"}, {"type": "text", "text": "。"}],
			"usage": {"input_tokens": 5, "output_tokens": 3}
		}`))
	}))
	defer server.Close()

	gateway := NewGateway()
	resp, err := gateway.Chat(context.Background(), ProviderConfig{
		Type: domain.ProviderAnthropic, APIBase: server.URL, APIKey: "sk-ant", ModelName: "claude-3-5-sonnet",
	}, ChatRequest{
		Messages: []Message{
			{Role: "system", Content: "你是编辑"},
			{Role: "user", Content: "在吗"},
		},
		TimeoutSec: 10,
	})
	if err != nil {
		t.Fatalf("调用失败: %v", err)
	}
	if resp.Content != "收到。" {
		t.Fatalf("多段文本应拼接，实际 %q", resp.Content)
	}
	if resp.TotalTokens != 8 {
		t.Errorf("总 token 应为 input+output=8，实际 %d", resp.TotalTokens)
	}
	if keyHeader != "sk-ant" || versionHeader != anthropicVersion {
		t.Errorf("鉴权头不正确: %q / %q", keyHeader, versionHeader)
	}
	if captured["system"] != "你是编辑" {
		t.Errorf("system 消息应抽到独立字段，实际 %v", captured["system"])
	}
	msgs, _ := captured["messages"].([]any)
	if len(msgs) != 1 {
		t.Errorf("messages 里应只剩非 system 消息，实际 %d 条", len(msgs))
	}
	if mt, _ := captured["max_tokens"].(float64); mt <= 0 {
		t.Errorf("Anthropic 的 max_tokens 必填，实际 %v", captured["max_tokens"])
	}
}

func TestChatTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"太慢"}}]}`))
	}))
	defer server.Close()

	gateway := NewGateway()
	_, err := gateway.Chat(context.Background(), ProviderConfig{
		Type: domain.ProviderOpenAICompatible, APIBase: server.URL, APIKey: "k", ModelName: "m",
	}, ChatRequest{Messages: []Message{{Role: "user", Content: "hi"}}, TimeoutSec: 0})
	if err != nil {
		t.Fatalf("默认超时（120s）下不应失败: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := gateway.Chat(ctx, ProviderConfig{
		Type: domain.ProviderOpenAICompatible, APIBase: server.URL, APIKey: "k", ModelName: "m",
	}, ChatRequest{Messages: []Message{{Role: "user", Content: "hi"}}, TimeoutSec: 5}); err == nil {
		t.Fatal("上下文取消时应报错")
	}
}

func TestChatValidatesConfig(t *testing.T) {
	gateway := NewGateway()
	cases := []struct {
		name string
		cfg  ProviderConfig
		req  ChatRequest
	}{
		{"类型非法", ProviderConfig{Type: "BOGUS", APIBase: "http://x", ModelName: "m"}, ChatRequest{Messages: []Message{{Role: "user", Content: "x"}}}},
		{"缺接口地址", ProviderConfig{Type: domain.ProviderOpenAICompatible, ModelName: "m"}, ChatRequest{Messages: []Message{{Role: "user", Content: "x"}}}},
		{"缺模型名", ProviderConfig{Type: domain.ProviderOpenAICompatible, APIBase: "http://x"}, ChatRequest{Messages: []Message{{Role: "user", Content: "x"}}}},
		{"缺消息", ProviderConfig{Type: domain.ProviderOpenAICompatible, APIBase: "http://x", ModelName: "m"}, ChatRequest{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := gateway.Chat(context.Background(), tc.cfg, tc.req); err == nil {
				t.Fatal("应报错")
			}
		})
	}
}
