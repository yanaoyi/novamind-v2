package retrieval

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/yanaoyi/novamindv2/backend/internal/domain"
)

type fakeStore struct {
	calls []struct {
		refKind string
		refID   *string
		chunks  []domain.RetrievalChunk
	}
	failOn string // 命中的 refKind 直接报错，用于验证错误不被吞
}

func (f *fakeStore) ReplaceChunks(
	_ context.Context,
	_, _, _, refKind string,
	refID *string, _ *string,
	chunks []domain.RetrievalChunk,
) error {
	if f.failOn == refKind {
		return errors.New("模拟写入失败")
	}
	f.calls = append(f.calls, struct {
		refKind string
		refID   *string
		chunks  []domain.RetrievalChunk
	}{refKind: refKind, refID: refID, chunks: chunks})
	return nil
}

// PruneChunks 只满足接口：索引编排本身不做剪枝（剪枝由 service 层在全量重建后调用）。
func (f *fakeStore) PruneChunks(context.Context, string, string, string, string, []string) error {
	return nil
}

func TestIndexWorkSplitsByKindAndReplaces(t *testing.T) {
	store := &fakeStore{}
	chapterID := "ch-1"
	longChapter := strings.Repeat("字", 2000)
	items := []SourceItem{
		{RefKind: domain.ChunkRefChapter, RefID: chapterID, ChapterID: &chapterID, Text: longChapter},
		{RefKind: domain.ChunkRefWorldRule, RefID: "rule-1", Text: "老城区的人情规则：先讲人情再讲道理。"},
	}
	got, err := IndexWork(context.Background(), store, "u1", domain.WorkKindCreative, "w1", items)
	if err != nil {
		t.Fatalf("索引失败: %v", err)
	}
	if got.Items != 2 {
		t.Fatalf("应索引 2 条来源，实际 %d", got.Items)
	}
	if len(store.calls) != 2 {
		t.Fatalf("应调用仓储 2 次，实际 %d", len(store.calls))
	}
	if chapterChunks := len(store.calls[0].chunks); chapterChunks < 3 {
		t.Errorf("2000 字章节应切成多块，实际 %d 块", chapterChunks)
	}
	if ruleChunks := len(store.calls[1].chunks); ruleChunks != 1 {
		t.Errorf("世界规则应单条即一块，实际 %d 块", ruleChunks)
	}
	if got.ByKind[domain.ChunkRefChapter] < 3 || got.ByKind[domain.ChunkRefWorldRule] != 1 {
		t.Errorf("按来源的统计不正确: %+v", got.ByKind)
	}
}

func TestIndexWorkClearsEmptySource(t *testing.T) {
	store := &fakeStore{}
	got, err := IndexWork(context.Background(), store, "u1", domain.WorkKindCreative, "w1", []SourceItem{
		{RefKind: domain.ChunkRefChapter, RefID: "ch-empty", Text: "   \n\n "},
	})
	if err != nil {
		t.Fatalf("索引失败: %v", err)
	}
	if got.Cleared != 1 || got.Chunks != 0 {
		t.Fatalf("空来源应记为清空，实际 %+v", got)
	}
	if len(store.calls) != 1 || len(store.calls[0].chunks) != 0 {
		t.Fatal("空来源必须调用仓储清空旧块（否则旧内容会永远留在索引里）")
	}
}

func TestIndexWorkStopsOnError(t *testing.T) {
	store := &fakeStore{failOn: domain.ChunkRefWorldRule}
	_, err := IndexWork(context.Background(), store, "u1", domain.WorkKindCreative, "w1", []SourceItem{
		{RefKind: domain.ChunkRefChapter, RefID: "ch-1", Text: "正文"},
		{RefKind: domain.ChunkRefWorldRule, RefID: "rule-1", Text: "规则"},
	})
	if err == nil {
		t.Fatal("写入失败必须返回错误（交给任务框架重试），不能吞掉")
	}
	if !strings.Contains(err.Error(), "world_rule") {
		t.Errorf("错误信息应指出失败的来源，实际: %v", err)
	}
}
