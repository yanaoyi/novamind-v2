package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/yanaoyi/novamindv2/backend/internal/ai"
)

// ModelInvoker 把「取默认模型 → 渲染 Prompt → 调网关」串成一步，供各 Agent/任务复用。
// 业务代码因此不需要知道用的是哪家模型、也不需要自己拼 HTTP。
type ModelInvoker struct {
	providers *ModelProviderService
	prompts   *ai.Engine
	gateway   *ai.Gateway
}

// NewModelInvoker 构建调用器。
func NewModelInvoker(providers *ModelProviderService, prompts *ai.Engine, gateway *ai.Gateway) *ModelInvoker {
	return &ModelInvoker{providers: providers, prompts: prompts, gateway: gateway}
}

// RunPrompt 用指定模板跑一次模型调用，返回模型输出的原始文本。
// promptName 用 <name>（自动取最新版本）；JSONMode 默认开启（规格书 §57：输出必须结构化）。
func (m *ModelInvoker) RunPrompt(ctx context.Context, promptName string, data any) (string, error) {
	if m.prompts == nil {
		return "", errors.New("Prompt 引擎未初始化")
	}
	prompt, err := m.prompts.Get(promptName, "")
	if err != nil {
		return "", err
	}
	rendered, err := prompt.Render(data)
	if err != nil {
		return "", err
	}

	cfg, provider, err := m.providers.ResolveRuntime(ctx, "")
	if err != nil {
		return "", err
	}
	resp, err := m.gateway.Chat(ctx, cfg, ai.ChatRequest{
		Messages: []ai.Message{
			{Role: "system", Content: "你是严谨的中文小说分析助手。只输出要求的 JSON，不要输出任何解释文字。"},
			{Role: "user", Content: rendered},
		},
		Temperature: provider.Temperature,
		MaxTokens:   provider.MaxTokens,
		JSONMode:    true,
		TimeoutSec:  provider.TimeoutSec,
	})
	if err != nil {
		return "", err
	}
	if resp.Content == "" {
		return "", fmt.Errorf("模型返回了空内容（prompt=%s）", prompt.Name)
	}
	return resp.Content, nil
}

// PromptNames 返回可用模板名（便于界面展示与自检）。
func (m *ModelInvoker) PromptNames() []string {
	if m.prompts == nil {
		return []string{}
	}
	metas := m.prompts.List()
	names := make([]string, 0, len(metas))
	for _, meta := range metas {
		names = append(names, meta.Name)
	}
	return names
}
