package task

import (
	"context"
	"errors"

	"github.com/yanaoyi/novamindv2/backend/internal/domain"
	"github.com/yanaoyi/novamindv2/backend/internal/retrieval"
	"github.com/yanaoyi/novamindv2/backend/internal/service"
)

// TypeIndexChunks 是检索索引重建任务（Phase 9 §9.1.4）。
const TypeIndexChunks = "index_chunks"

// RegisterIndexHandlers 注册索引任务：把作品内容切片写进 chunks。
//
// 入参 {"work_kind":"original|creative","work_id":"<uuid>","ref_kind":"<可选>","ref_id":"<可选>"}。
// ref 为空 = 全量重建；给 ref = 只重建这一个来源（增量，Phase 9 §9.1.4）。
// 失败由任务框架按既有退避策略重试；索引不在写作主链路上，失败不影响写作。
func RegisterIndexHandlers(reg *Registry, indexer *service.IndexService) {
	reg.Register(TypeIndexChunks, func(ctx context.Context, t domain.Task, r Reporter) (map[string]any, error) {
		workKind := InputString(t.Input, "work_kind")
		workID := InputString(t.Input, "work_id")
		if workKind == "" || workID == "" {
			return nil, errors.New("任务缺少 work_kind 或 work_id")
		}
		r.Progress(10, "准备索引")
		refKind := InputString(t.Input, "ref_kind")
		refID := InputString(t.Input, "ref_id")
		var (
			result *retrieval.IndexResult
			err    error
		)
		if refKind != "" && refID != "" {
			result, err = indexer.IndexRef(ctx, workKind, workID, refKind, refID)
		} else {
			result, err = indexer.IndexWork(ctx, workKind, workID)
		}
		if err != nil {
			return nil, err
		}
		if r.Cancelled() {
			return nil, errors.New("任务已取消")
		}
		r.Progress(100, "索引完成")
		return map[string]any{
			"work_kind": workKind,
			"work_id":   workID,
			"ref_kind":  refKind,
			"ref_id":    refID,
			"items":     result.Items,
			"chunks":    result.Chunks,
			"by_kind":   result.ByKind,
			"cleared":   result.Cleared,
		}, nil
	})
}
