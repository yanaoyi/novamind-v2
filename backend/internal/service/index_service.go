package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/yanaoyi/novamindv2/backend/internal/domain"
	"github.com/yanaoyi/novamindv2/backend/internal/repository"
	"github.com/yanaoyi/novamindv2/backend/internal/retrieval"
)

// IndexWorkReader 取原著章节正文（由 OriginalRepo 实现）。
type IndexWorkReader interface {
	ListChapterContents(ctx context.Context, workID string, limit int) ([]domain.OriginalChapter, error)
}

// CreativeChapterReader 取二创章节正文（由 WritingRepo 实现）。
type CreativeChapterReader interface {
	ListChapters(ctx context.Context, workID string, withContent bool) ([]domain.CreativeChapter, error)
}

// IndexService 把作品内容索引进检索分块表（Phase 9 §9.1.4）。
//
// 先覆盖"章节正文"这条主链路（原著 + 二创）：检索最常命中的就是正文，
// 9.6 剧本考的"第 4 章召回第 1 章伏笔"也完全落在这条链路上。
// 大纲节点 / 世界规则 / 人物 / 事件的取数随后补齐（同一套 IndexWork 编排）。
type IndexService struct {
	store     retrieval.ChunkStore
	users     *repository.UserRepo
	originals IndexWorkReader
	creative  CreativeChapterReader
}

// NewIndexService 构建服务。
func NewIndexService(
	store retrieval.ChunkStore,
	users *repository.UserRepo,
	originals IndexWorkReader,
	creative CreativeChapterReader,
) *IndexService {
	return &IndexService{store: store, users: users, originals: originals, creative: creative}
}

// IndexWork 全量重建某部作品的分块。
func (s *IndexService) IndexWork(ctx context.Context, workKind, workID string) (*retrieval.IndexResult, error) {
	ownerID, err := s.users.DefaultUserID(ctx)
	if err != nil {
		return nil, err
	}
	var items []retrieval.SourceItem
	switch strings.TrimSpace(workKind) {
	case domain.WorkKindOriginal:
		chapters, err := s.originals.ListChapterContents(ctx, workID, 0)
		if err != nil {
			return nil, err
		}
		for _, c := range chapters {
			items = append(items, retrieval.SourceItem{
				RefKind: domain.ChunkRefChapter, RefID: c.ID, Text: c.Content,
			})
		}
	case domain.WorkKindCreative:
		chapters, err := s.creative.ListChapters(ctx, workID, true)
		if err != nil {
			return nil, err
		}
		for _, c := range chapters {
			id := c.ID
			items = append(items, retrieval.SourceItem{
				RefKind: domain.ChunkRefChapter, RefID: id, ChapterID: &id, Text: c.Content,
			})
		}
	default:
		return nil, fmt.Errorf("%w：work_kind 只能是 original 或 creative", ErrBadRequest)
	}
	return retrieval.IndexWork(ctx, s.store, ownerID, workKind, workID, items)
}
