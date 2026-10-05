package service

import (
	"context"
	"fmt"

	"github.com/yanaoyi/novamindv2/backend/internal/domain"
	"github.com/yanaoyi/novamindv2/backend/internal/repository"
)

// SnapshotService 管理上下文快照的读取与写入（Phase 9 §9.2.2）。
//
// 语义：**只增不改**。写入发生在每次 AI 调用之前（先落快照再调模型），
// 读取只用于"当时到底给了 AI 什么"的追溯；没有任何修改/删除入口。
type SnapshotService struct {
	repo     *repository.ContextSnapshotRepo
	creative CreativeWorkReader
	writing  *WritingService
	users    *repository.UserRepo
}

// NewSnapshotService 构建服务。
func NewSnapshotService(
	repo *repository.ContextSnapshotRepo,
	creative CreativeWorkReader,
	writing *WritingService,
	users *repository.UserRepo,
) *SnapshotService {
	return &SnapshotService{repo: repo, creative: creative, writing: writing, users: users}
}

// Record 写入一条快照（在调用模型之前）。
//
// chapterID 为空表示"不针对具体章节"的调用（例如一致性检查整本作品）；
// kind 必须合法，否则调用方就写错了，直接报错而不是默默落下一条不可用的记录。
func (s *SnapshotService) Record(
	ctx context.Context,
	chapterID *string,
	kind domain.SnapshotKind,
	snapshot map[string]any,
) (*domain.ContextSnapshot, error) {
	if !kind.Valid() {
		return nil, fmt.Errorf("%w: %s", ErrBadRequest, kind)
	}
	var workID string
	if chapterID != nil {
		chapter, err := s.writing.GetChapter(ctx, *chapterID)
		if err != nil {
			return nil, err
		}
		workID = chapter.CreativeWorkID
	} else {
		// 没有章节时从 snapshot 里取作品 id（调用方必须带上，否则无从归属）
		if v, ok := snapshot["creative_work_id"].(string); ok {
			workID = v
		}
	}
	if workID == "" {
		return nil, fmt.Errorf("%w：快照缺少作品归属（chapter_id 或 creative_work_id）", ErrBadRequest)
	}
	if _, err := s.creative.GetWorkByID(ctx, workID); err != nil {
		return nil, err
	}
	ownerID, err := s.users.DefaultUserID(ctx)
	if err != nil {
		return nil, err
	}
	record := &domain.ContextSnapshot{
		OwnerUserID: ownerID, CreativeWorkID: workID, ChapterID: chapterID,
		Kind: kind, Snapshot: snapshot,
	}
	if err := s.repo.Create(ctx, record); err != nil {
		return nil, err
	}
	return record, nil
}

// ListByChapter 列出某章节的快照（新到旧，不带 payload）。
func (s *SnapshotService) ListByChapter(ctx context.Context, chapterID string) ([]domain.ContextSnapshot, error) {
	if _, err := s.writing.GetChapter(ctx, chapterID); err != nil {
		return nil, err
	}
	items, err := s.repo.ListByChapter(ctx, chapterID)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []domain.ContextSnapshot{}
	}
	return items, nil
}

// Get 取快照详情（含 payload）。
func (s *SnapshotService) Get(ctx context.Context, id string) (*domain.ContextSnapshot, error) {
	return s.repo.GetByID(ctx, id)
}
