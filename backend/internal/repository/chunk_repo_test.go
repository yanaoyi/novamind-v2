package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/yanaoyi/novamindv2/backend/internal/domain"
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
