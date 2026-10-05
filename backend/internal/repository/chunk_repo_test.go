package repository

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/yanaoyi/novamindv2/backend/internal/domain"
	"github.com/yanaoyi/novamindv2/backend/internal/retrieval"
)

func newChunkRepoForTest(t *testing.T) (*ChunkRepo, string, string) {
	t.Helper()
	repo := newTestRepo(t)
	userID, err := NewUserRepo(repo.db).DefaultUserID(context.Background())
	if err != nil {
		t.Fatalf("取默认用户失败（迁移 0018 应已建好 default 账号）: %v", err)
	}
	// chunks.work_id 故意没有外键（跨 original/creative 两表），所以这里用随机 UUID 即可，
	// 不需要为测试造一部作品出来
	return NewChunkRepo(repo.db), userID, uuid.NewString()
}

func TestChunkRepoReplaceIsIdempotent(t *testing.T) {
	ctx := context.Background()
	repo, userID, workID := newChunkRepoForTest(t)
	refID := workID

	first := []domain.RetrievalChunk{
		{Seq: 0, Content: "第一块：沈砚回到老宅。", TokenCount: 10},
		{Seq: 1, Content: "第二块：账册上有第二笔钱。", TokenCount: 12},
	}
	if err := repo.ReplaceChunks(ctx, userID, domain.WorkKindCreative, workID, domain.ChunkRefChapter, &refID, &refID, first); err != nil {
		t.Fatalf("首次写入失败: %v", err)
	}
	if n, _ := repo.CountByWork(ctx, domain.WorkKindCreative, workID); n != 2 {
		t.Fatalf("应有 2 块，实际 %d", n)
	}

	// 再写一次不同内容：必须替换而不是叠加（幂等）
	second := []domain.RetrievalChunk{{Seq: 0, Content: "替换后的唯一一块。", TokenCount: 8}}
	if err := repo.ReplaceChunks(ctx, userID, domain.WorkKindCreative, workID, domain.ChunkRefChapter, &refID, &refID, second); err != nil {
		t.Fatalf("二次写入失败: %v", err)
	}
	if n, _ := repo.CountByWork(ctx, domain.WorkKindCreative, workID); n != 1 {
		t.Fatalf("替换后应剩 1 块，实际 %d", n)
	}

	// 传空切片 = 删除该来源的全部分块（章节被清空时的语义）
	if err := repo.ReplaceChunks(ctx, userID, domain.WorkKindCreative, workID, domain.ChunkRefChapter, &refID, &refID, nil); err != nil {
		t.Fatalf("清空失败: %v", err)
	}
	if n, _ := repo.CountByWork(ctx, domain.WorkKindCreative, workID); n != 0 {
		t.Fatalf("清空后应 0 块，实际 %d", n)
	}
}

// 端到端（真库）：分块写进 chunks → 用仓储当数据源跑 BM25 → 相关块排在前面。
// 这一步证明"检索能真的从库里召回"，而不只是纯函数的单元测试。
func TestChunkRepoSearchEndToEnd(t *testing.T) {
	ctx := context.Background()
	repo, userID, workID := newChunkRepoForTest(t)
	refID := workID
	chunks := []domain.RetrievalChunk{
		{Seq: 0, Content: "雨夜。沈砚翻开账册，发现第二笔银子的去向不对劲。", TokenCount: 24},
		{Seq: 1, Content: "清晨的集市很热闹，卖鱼的老汉在吆喝。", TokenCount: 18},
	}
	if err := repo.ReplaceChunks(ctx, userID, domain.WorkKindCreative, workID,
		domain.ChunkRefChapter, &refID, &refID, chunks); err != nil {
		t.Fatalf("写入分块失败: %v", err)
	}
	got, err := retrieval.Search(ctx, repo, userID, domain.WorkKindCreative, workID, "账册 银子", 5)
	if err != nil {
		t.Fatalf("检索失败: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("应从库里召回至少一块")
	}
	if !strings.Contains(got[0].Content, "账册") {
		t.Fatalf("首条结果应包含查询词，实际: %s", got[0].Content)
	}
	if got[0].RefKind != domain.ChunkRefChapter {
		t.Errorf("命中结果应带回来源信息，实际 ref_kind=%s", got[0].RefKind)
	}
	// owner 隔离：换一个不存在的 owner 检索，必须召回不到这部作品的块
	others, err := retrieval.Search(ctx, repo, "00000000-0000-7000-8000-0000000000ff", domain.WorkKindCreative, workID, "账册", 5)
	if err != nil {
		t.Fatalf("隔离检索出错: %v", err)
	}
	if len(others) != 0 {
		t.Fatalf("不同 owner 不应召回同一作品的块，实际 %d 条", len(others))
	}
}
