package retrieval

import (
	"context"
	"strings"
	"testing"
)

type fakeSource struct{ chunks []IndexedChunk }

func (f *fakeSource) ListChunks(_ context.Context, _, _, _ string) ([]IndexedChunk, error) {
	return f.chunks, nil
}

func TestTokenizeCharBigram(t *testing.T) {
	got := tokenize("长篇记忆，回来了！")
	want := []string{"长篇", "篇记", "记忆", "回来", "来了"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("分词不正确: 得到 %v 期望 %v", got, want)
	}
	single := tokenize("剑")
	if len(single) != 1 || single[0] != "剑" {
		t.Fatalf("单字应保留，实际 %v", single)
	}
	across := tokenize("甲，乙")
	if len(across) != 2 || across[0] != "甲" || across[1] != "乙" {
		t.Fatalf("标点是硬边界：不应跨标点组 bigram，实际 %v", across)
	}
}

func TestSearchRanksRelevantChunkFirst(t *testing.T) {
	src := &fakeSource{chunks: []IndexedChunk{
		{ID: "c1", Content: "雨夜。沈砚翻开账册，发现第二笔银子的去向不对劲。"},
		{ID: "c2", Content: "清晨的集市很热闹，卖鱼的老汉在吆喝。"},
		{ID: "c3", Content: "老宅的门虚掩着，账册就放在八仙桌上。"},
	}}
	got, err := Search(context.Background(), src, "u1", "creative", "w1", "账册 银子", 3)
	if err != nil {
		t.Fatalf("检索失败: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("应有命中结果")
	}
	if got[0].ID != "c1" {
		t.Fatalf("最相关的应是 c1（同时含账册与银子），实际 %s", got[0].ID)
	}
	if len(got[0].MatchFrom) == 0 || got[0].MatchFrom[0] != "bm25" {
		t.Errorf("命中来源应标记 bm25，实际 %v", got[0].MatchFrom)
	}
	for _, item := range got {
		if item.ID == "c2" {
			t.Errorf("不相干的块不应入选: %+v", item)
		}
	}
}

func TestSearchEmptyQueryAndEmptyIndex(t *testing.T) {
	src := &fakeSource{chunks: []IndexedChunk{{ID: "c1", Content: "任意内容"}}}
	if got, err := Search(context.Background(), src, "u1", "creative", "w1", "，。！", 5); err != nil || len(got) != 0 {
		t.Fatalf("空查询应返回空结果，实际 %v / %v", got, err)
	}
	empty := &fakeSource{}
	if got, err := Search(context.Background(), empty, "u1", "creative", "w1", "账册", 5); err != nil || len(got) != 0 {
		t.Fatalf("空索引应返回空结果，实际 %v / %v", got, err)
	}
}

func TestFuseRRFMergesAndDedupes(t *testing.T) {
	bm25 := []ScoredChunk{
		{IndexedChunk: IndexedChunk{ID: "a", Content: "A"}, MatchFrom: []string{"bm25"}},
		{IndexedChunk: IndexedChunk{ID: "b", Content: "B"}, MatchFrom: []string{"bm25"}},
	}
	vector := []ScoredChunk{
		{IndexedChunk: IndexedChunk{ID: "b", Content: "B"}, MatchFrom: []string{"vector"}},
		{IndexedChunk: IndexedChunk{ID: "c", Content: "C"}, MatchFrom: []string{"vector"}},
	}
	got := FuseRRF(bm25, vector)
	if len(got) != 3 {
		t.Fatalf("两路融合后应有 3 个不同块（去重），实际 %d", len(got))
	}
	if got[0].ID != "b" {
		t.Fatalf("两路都命中的 b 应排第一（RRF 分数相加），实际 %s", got[0].ID)
	}
	if len(got[0].MatchFrom) != 2 {
		t.Errorf("b 应标记来自两路，实际 %v", got[0].MatchFrom)
	}
}
