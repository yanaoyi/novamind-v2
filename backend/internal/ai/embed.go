package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// EmbedBatchSize 是单次 /embeddings 调用的最大输入条数（任务书要求 ≤32）。
const EmbedBatchSize = 32

type openAIEmbedRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type openAIEmbedResponse struct {
	Data []struct {
		Index     int       `json:"index"`
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
	Model string `json:"model"`
	Usage struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Embed 调 OpenAI 兼容的 /embeddings 取向量（Phase 9 §9.1.3）。
//
// 约定：
//   - cfg.EmbedAPIBase 为空时回退 cfg.APIBase，cfg.EmbedModelName 由调用方给定（默认 BGE-large-zh 量级）；
//   - 输入按 EmbedBatchSize 分批，批间不并发（避免打爆上游额度），失败复用与 chat 相同的重试语义；
//   - 返回顺序与输入**严格一致**：按响应里的 index 排序后拼装，不依赖上游返回顺序。
func (g *Gateway) Embed(ctx context.Context, cfg ProviderConfig, inputs []string) ([][]float32, error) {
	if len(inputs) == 0 {
		return [][]float32{}, nil
	}
	base := strings.TrimSpace(cfg.EmbedAPIBase)
	if base == "" {
		base = cfg.APIBase
	}
	if strings.TrimSpace(base) == "" {
		return nil, fmt.Errorf("embedding 接口地址为空（embed_api_base 与 api_base 都没配）")
	}
	model := strings.TrimSpace(cfg.EmbedModelName)
	if model == "" {
		return nil, fmt.Errorf("embedding 模型名为空（embed_model_name 未配置）")
	}
	timeout := time.Duration(cfg.TimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 120 * time.Second
	}

	out := make([][]float32, 0, len(inputs))
	for start := 0; start < len(inputs); start += EmbedBatchSize {
		end := start + EmbedBatchSize
		if end > len(inputs) {
			end = len(inputs)
		}
		vectors, err := g.embedBatch(ctx, cfg, base, model, inputs[start:end], timeout)
		if err != nil {
			return nil, fmt.Errorf("第 %d-%d 条 embedding 失败: %w", start+1, end, err)
		}
		out = append(out, vectors...)
	}
	if len(out) != len(inputs) {
		return nil, fmt.Errorf("embedding 数量不匹配：输入 %d 条，返回 %d 条", len(inputs), len(out))
	}
	return out, nil
}

func (g *Gateway) embedBatch(
	ctx context.Context,
	cfg ProviderConfig,
	base, model string,
	batch []string,
	timeout time.Duration,
) ([][]float32, error) {
	body := openAIEmbedRequest{Model: model, Input: batch}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("构造 embedding 请求失败: %w", err)
	}
	endpoint := strings.TrimRight(base, "/") + "/embeddings"
	if _, err := url.Parse(endpoint); err != nil {
		return nil, fmt.Errorf("embedding 接口地址非法: %w", err)
	}

	var lastErr error
	for attempt := 1; attempt <= g.maxRetry; attempt++ {
		attemptCtx, cancel := context.WithTimeout(ctx, timeout)
		req, err := http.NewRequestWithContext(attemptCtx, http.MethodPost, endpoint, bytes.NewReader(payload))
		if err != nil {
			cancel()
			return nil, fmt.Errorf("构造 HTTP 请求失败: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		if cfg.APIKey != "" {
			req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
		}
		resp, err := g.httpClient.Do(req)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("调用 embedding 接口失败: %w", err)
		}
		raw, readErr := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
		resp.Body.Close()
		cancel()
		if readErr != nil {
			return nil, fmt.Errorf("读取 embedding 响应失败: %w", readErr)
		}
		if resp.StatusCode != http.StatusOK {
			apiErr := &APIError{
				Provider: string(cfg.Type) + "/embeddings", StatusCode: resp.StatusCode, Body: string(raw),
			}
			lastErr = apiErr
			if !apiErr.IsRetryable() {
				return nil, apiErr
			}
			time.Sleep(time.Duration(attempt) * 800 * time.Millisecond)
			continue
		}
		var parsed openAIEmbedResponse
		if err := json.Unmarshal(raw, &parsed); err != nil {
			return nil, fmt.Errorf("解析 embedding 响应失败: %w（前 200 字：%s）", err, head(string(raw), 200))
		}
		if parsed.Error != nil {
			return nil, fmt.Errorf("embedding 接口返回错误: %s", parsed.Error.Message)
		}
		if len(parsed.Data) != len(batch) {
			return nil, fmt.Errorf("embedding 返回条数不符：期望 %d，实际 %d", len(batch), len(parsed.Data))
		}
		sort.Slice(parsed.Data, func(i, j int) bool { return parsed.Data[i].Index < parsed.Data[j].Index })
		out := make([][]float32, 0, len(parsed.Data))
		for _, item := range parsed.Data {
			if len(item.Embedding) == 0 {
				return nil, fmt.Errorf("第 %d 条 embedding 为空", item.Index)
			}
			out = append(out, item.Embedding)
		}
		return out, nil
	}
	return nil, lastErr
}
