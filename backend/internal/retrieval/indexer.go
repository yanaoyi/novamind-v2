package retrieval

import (
	"context"
	"fmt"

	"github.com/yanaoyi/novamindv2/backend/internal/domain"
)

// ChunkStore 是索引编排需要的写入能力（由 repository.ChunkRepo 实现）。
type ChunkStore interface {
	ReplaceChunks(
		ctx context.Context,
		ownerUserID, workKind, workID, refKind string,
		refID *string, chapterID *string,
		chunks []domain.RetrievalChunk,
	) error
	// PruneChunks 删掉该 (作品, 来源类型) 下**不在 keepRefIDs 里**的旧块。
	//
	// 为什么必须有：全量重建时我们只会 ReplaceChunks 现存来源，
	// 被删除的章节 / 被移除的世界规则 / 被删掉的大纲节点如果不剪，
	// 它们的旧块会永远留在索引里 —— 检索会召回已经不存在的设定。
	PruneChunks(
		ctx context.Context,
		ownerUserID, workKind, workID, refKind string,
		keepRefIDs []string,
	) error
}

// SourceItem 是待索引的一条来源（章节正文 / 大纲节点 / 世界规则 / 人物 / 事件 / 记忆事实…）。
//
// 调用方（任务 handler）负责从各表取内容，本包只负责"怎么切、怎么落"，
// 这样索引逻辑可以脱离数据库单测。
type SourceItem struct {
	RefKind string
	RefID   string
	// ChapterID 只有章节类来源才有（用于回溯"这段来自哪一章"）
	ChapterID *string
	Text      string
}

// IndexResult 是一次索引的结果统计（用于任务输出与冒烟校验）。
type IndexResult struct {
	Items  int            `json:"items"`
	Chunks int            `json:"chunks"`
	ByKind map[string]int `json:"by_kind"`
	// Cleared 记录"内容为空因此清空了该来源旧块"的条数
	Cleared int `json:"cleared"`
}

// IndexWork 增量重建一批来源的分块：逐条切片 → 幂等替换该来源的旧块。
//
// 语义要点（与任务书一致）：
//   - 章节按段落窗口切（≤800 字、重叠 150）；其它来源单条即一块（超长仍会切）；
//   - 文本为空 = 该来源的块全部清掉，而不是"什么都不做" —— 否则章节被清空后，
//     旧块会永远留在索引里，检索召回的内容与正文不一致；
//   - 任一来源写入失败直接返回错误，交给任务框架重试（不吞错、不半途静默）。
func IndexWork(
	ctx context.Context,
	store ChunkStore,
	ownerUserID, workKind, workID string,
	items []SourceItem,
) (*IndexResult, error) {
	result := &IndexResult{ByKind: map[string]int{}}
	for _, item := range items {
		if item.RefKind == "" {
			return nil, fmt.Errorf("索引来源缺少 ref_kind（ref_id=%s）", item.RefID)
		}
		var chunks []domain.RetrievalChunk
		if item.RefKind == domain.ChunkRefChapter {
			chunks = ChunkChapter(item.Text)
		} else {
			chunks = ChunkSingle(item.Text)
		}
		var refID *string
		if item.RefID != "" {
			id := item.RefID
			refID = &id
		}
		if err := store.ReplaceChunks(ctx, ownerUserID, workKind, workID, item.RefKind, refID, item.ChapterID, chunks); err != nil {
			return nil, fmt.Errorf("索引 %s/%s 失败: %w", item.RefKind, item.RefID, err)
		}
		result.Items++
		result.Chunks += len(chunks)
		result.ByKind[item.RefKind] += len(chunks)
		if len(chunks) == 0 {
			result.Cleared++
		}
	}
	return result, nil
}
