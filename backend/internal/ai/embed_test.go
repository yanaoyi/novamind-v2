package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yanaoyi/novamindv2/backend/internal/domain"
)

// 按第 index 条返回向量；顺带统计收到的批次数与每批条数，用于验证分批行为。
func newEmbedServer(t *testing.T, calls *int, batchSizes *[]int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/embeddings" {
			t.Errorf("路径应为 /v1/embeddings，实际 %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer sk-test" {
			t.Errorf("缺少鉴权头，实际 %q", got)
		}
		var req openAIEmbedRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		*calls++
		*batchSizes = append(*batchSizes, len(req.Input))
		data := make([]map[string]any, 0, len(req.Input))
		for i := range req.Input {
			data = append(data, map[string]any{
				"index":     i,
				"embedding": []float32{float32(i) + 0.5, 0.25},
			})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": data, "model": req.Model, "usage": map[string]any{"total_tokens": len(req.Input) * 3},
		})
	}))
}

func TestEmbedBatchesAndKeepsOrder(t *testing.T) {
	var calls int
	var batchSizes []int
	srv := newEmbedServer(t, &calls, &batchSizes)
	defer srv.Close()

	inputs := make([]string, 33) // 33 条 → 应分成 32 + 1 两批
	for i := range inputs {
		inputs[i] = "文本"
	}
	gw := NewGateway()
	vectors, err := gw.Embed(context.Background(), ProviderConfig{
		Type: domain.ProviderOpenAICompatible, APIBase: srv.URL + "/v1", APIKey: "sk-test",
		EmbedModelName: "bge-large-zh-v1.5", TimeoutSec: 10,
	}, inputs)
	if err != nil {
		t.Fatalf("Embed 失败: %v", err)
	}
	if calls != 2 {
		t.Errorf("33 条应分 2 批，实际 %d 批", calls)
	}
	if len(vectors) != 33 {
		t.Fatalf("应返回 33 个向量，实际 %d", len(vectors))
	}
	// 顺序严格对齐：第 i 条的向量首维是 i+0.5（下标 5 → 5.5）
	if vectors[5][0] != 5.5 {
		t.Errorf("向量顺序错乱：第 5 条首维应为 5.5，实际 %v", vectors[5][0])
	}
	if batchSizes[0] != EmbedBatchSize || batchSizes[1] != 1 {
		t.Errorf("分批大小应为 [32 1]，实际 %v", batchSizes)
	}
}

func TestEmbedValidation(t *testing.T) {
	gw := NewGateway()
	if _, err := gw.Embed(context.Background(), ProviderConfig{Type: domain.ProviderOpenAICompatible, APIBase: "http://x"}, []string{"a"}); err == nil {
		t.Error("缺 embedding 模型名应报错（维度不同的模型不能猜）")
	}
	if _, err := gw.Embed(context.Background(), ProviderConfig{Type: domain.ProviderOpenAICompatible, EmbedModelName: "m"}, []string{"a"}); err == nil {
		t.Error("缺接口地址应报错")
	}
	got, err := gw.Embed(context.Background(), ProviderConfig{EmbedModelName: "m"}, nil)
	if err != nil || len(got) != 0 {
		t.Errorf("空输入应直接返回空，实际 %v / %v", got, err)
	}
}

func TestEmbedRetriesOnServerError(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":{"message":"上游抖动"}}`))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{{"index": 0, "embedding": []float32{1, 2}}},
		})
	}))
	defer srv.Close()

	gw := NewGateway()
	vectors, err := gw.Embed(context.Background(), ProviderConfig{
		Type: domain.ProviderOpenAICompatible, APIBase: srv.URL, APIKey: "sk-test",
		EmbedModelName: "m", TimeoutSec: 10,
	}, []string{"a"})
	if err != nil {
		t.Fatalf("5xx 应重试后成功，实际 %v", err)
	}
	if calls != 2 || len(vectors) != 1 {
		t.Errorf("应重试 1 次并返回 1 个向量，实际 calls=%d vectors=%d", calls, len(vectors))
	}
}
