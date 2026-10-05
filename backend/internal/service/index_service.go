package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/yanaoyi/novamindv2/backend/internal/domain"
	"github.com/yanaoyi/novamindv2/backend/internal/retrieval"
)

// IndexWorkReader 取原著章节正文（由 OriginalRepo 实现）。
type IndexWorkReader interface {
	// 索引必须全量：不要用带 limit 的 ListChapterContents（它的 limit<=0 会被当成默认 50 章）
	ListAllChapterContents(ctx context.Context, workID string) ([]domain.OriginalChapter, error)
}

// CreativeChapterReader 取二创章节正文（由 WritingRepo 实现）。
type CreativeChapterReader interface {
	ListChapters(ctx context.Context, workID string, withContent bool) ([]domain.CreativeChapter, error)
	// GetChapter 取单章（增量索引：只重建受影响的 ref，不为了一个 ref 把整本正文读一遍）
	GetChapter(ctx context.Context, id string) (*domain.CreativeChapter, error)
}

// OwnerResolver 给出分块的归属账号（由 repository.UserRepo 实现）。
// 抽成接口是为了让索引逻辑能脱离数据库单测（原先直接依赖具体仓储类型，测不了）。
type OwnerResolver interface {
	DefaultUserID(ctx context.Context) (string, error)
}

// IndexService 把作品内容索引进检索分块表（Phase 9 §9.1.4）。
//
// 先覆盖"章节正文"这条主链路（原著 + 二创）：检索最常命中的就是正文，
// 9.6 剧本考的"第 4 章召回第 1 章伏笔"也完全落在这条链路上。
// 大纲节点 / 世界规则 / 人物 / 事件的取数随后补齐（同一套 IndexWork 编排）。
type IndexService struct {
	store     retrieval.ChunkStore
	users     OwnerResolver
	originals IndexWorkReader
	creative  CreativeChapterReader
}

// NewIndexService 构建服务。
func NewIndexService(
	store retrieval.ChunkStore,
	users OwnerResolver,
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
		chapters, err := s.originals.ListAllChapterContents(ctx, workID)
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

// IndexText 把一个"单条即一块"的来源写进检索索引（记忆事实 / 章节摘要用）。
//
// 空文本表示清空该来源的块 —— 记忆事实被新事实替代时就要这么做，
// 否则检索会同时召回"受伤"与"已痊愈"两条，凭空制造矛盾。
func (s *IndexService) IndexText(
	ctx context.Context,
	workKind, workID, refKind, refID, text string,
	chapterID *string,
) error {
	ownerID, err := s.users.DefaultUserID(ctx)
	if err != nil {
		return err
	}
	id := refID
	return s.store.ReplaceChunks(ctx, ownerID, workKind, workID, refKind, &id, chapterID,
		retrieval.ChunkSingle(text))
}

// IndexRef 只重建某一个来源（增量索引，§9.1.4 的"只重建受影响的 ref"）。
//
// 当前支持「二创 + 章节」——那是唯一会被反复修改的来源（作者编辑、AI 写本章）；
// 其余组合退回全量重建：语义等价、只是多做一点活，比"报错让任务失败"更符合
// "索引不该阻塞写作"的定位。
func (s *IndexService) IndexRef(ctx context.Context, workKind, workID, refKind, refID string) (*retrieval.IndexResult, error) {
	if strings.TrimSpace(refID) == "" {
		return s.IndexWork(ctx, workKind, workID)
	}
	if workKind != domain.WorkKindCreative || refKind != domain.ChunkRefChapter {
		fmt.Printf("[index] %s/%s 暂不支持按 ref 增量重建（ref=%s/%s），改为全量重建\n",
			workKind, workID, refKind, refID)
		return s.IndexWork(ctx, workKind, workID)
	}
	ownerID, err := s.users.DefaultUserID(ctx)
	if err != nil {
		return nil, err
	}
	chapter, err := s.creative.GetChapter(ctx, refID)
	if err != nil {
		// 章节被删掉时不该留下它的旧块（否则检索会召回已经不存在的正文）。
		if errors.Is(err, domain.ErrCreativeChapterNotFound) {
			return retrieval.IndexWork(ctx, s.store, ownerID, workKind, workID,
				[]retrieval.SourceItem{{RefKind: domain.ChunkRefChapter, RefID: refID}})
		}
		return nil, err
	}
	if chapter.CreativeWorkID != workID {
		return nil, fmt.Errorf("%w：章节不属于该二创作品", ErrBadRequest)
	}
	id := chapter.ID
	return retrieval.IndexWork(ctx, s.store, ownerID, workKind, workID,
		[]retrieval.SourceItem{{
			RefKind: domain.ChunkRefChapter, RefID: id, ChapterID: &id, Text: chapter.Content,
		}})
}
