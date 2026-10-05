package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
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

// CreativeSettingReader 取二创的设定类来源（人物 / 世界规则）。
//
// 由 CreativeService 实现：它已经提供这两个方法（写作链路也在用），
// 这里只为索引声明一个更窄的接口 —— 索引不该依赖整个二创服务。
type CreativeSettingReader interface {
	ListCharacters(ctx context.Context, workID string) ([]domain.CreativeCharacter, error)
	GetWorldDetail(ctx context.Context, workID string) (*WorldDetail, error)
}

// OutlineNodeReader 取大纲树（由 repository.OutlineRepo 实现）。
type OutlineNodeReader interface {
	ListOutlines(ctx context.Context, workID string) ([]domain.Outline, error)
	ListNodes(ctx context.Context, outlineID string) ([]domain.OutlineNode, error)
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
	settings  CreativeSettingReader
	outlines  OutlineNodeReader
}

// NewIndexService 构建服务。
func NewIndexService(
	store retrieval.ChunkStore,
	users OwnerResolver,
	originals IndexWorkReader,
	creative CreativeChapterReader,
	settings CreativeSettingReader,
	outlines OutlineNodeReader,
) *IndexService {
	return &IndexService{
		store: store, users: users, originals: originals,
		creative: creative, settings: settings, outlines: outlines,
	}
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
		creativeItems, err := s.collectCreative(ctx, workID)
		if err != nil {
			return nil, err
		}
		items = creativeItems
	default:
		return nil, fmt.Errorf("%w：work_kind 只能是 original 或 creative", ErrBadRequest)
	}
	result, err := retrieval.IndexWork(ctx, s.store, ownerID, workKind, workID, items)
	if err != nil {
		return nil, err
	}
	// 全量重建 = "索引必须恰好对应当前存在的来源"：剪掉已删除来源残留的旧块。
	// 只剪本服务负责的来源类型，不碰记忆事实/摘要（那是 MemoryService 的地盘）。
	for _, refKind := range indexedRefKinds(workKind) {
		if err := s.store.PruneChunks(ctx, ownerID, workKind, workID, refKind, refIDsOf(items, refKind)); err != nil {
			return nil, err
		}
	}
	return result, nil
}

// collectCreative 收集二创作品的全部来源：章节正文 + 人物 + 世界规则 + 大纲节点。
//
// 设定类来源（人物/规则/大纲）"单条即一 chunk"：它们本来就短，
// 而且检索到"某条规则"比检索到"某段正文"更有用（正文由章节路覆盖）。
func (s *IndexService) collectCreative(ctx context.Context, workID string) ([]retrieval.SourceItem, error) {
	chapters, err := s.creative.ListChapters(ctx, workID, true)
	if err != nil {
		return nil, err
	}
	items := make([]retrieval.SourceItem, 0, len(chapters))
	for _, c := range chapters {
		id := c.ID
		items = append(items, retrieval.SourceItem{
			RefKind: domain.ChunkRefChapter, RefID: id, ChapterID: &id, Text: c.Content,
		})
	}
	if s.settings != nil {
		if characters, err := s.settings.ListCharacters(ctx, workID); err == nil {
			for _, c := range characters {
				items = append(items, retrieval.SourceItem{
					RefKind: domain.ChunkRefCharacter, RefID: c.ID, Text: formatCharacterSource(c),
				})
			}
		}
		if detail, err := s.settings.GetWorldDetail(ctx, workID); err == nil && detail != nil {
			for _, rule := range detail.Rules {
				if rule.Status == domain.RuleRemoved {
					continue
				}
				items = append(items, retrieval.SourceItem{
					RefKind: domain.ChunkRefWorldRule, RefID: rule.ID,
					Text: formatWorldRuleSource(rule),
				})
			}
		}
	}
	if s.outlines != nil {
		outlines, err := s.outlines.ListOutlines(ctx, workID)
		if err == nil {
			for _, outline := range outlines {
				nodes, err := s.outlines.ListNodes(ctx, outline.ID)
				if err != nil {
					continue
				}
				for _, node := range nodes {
					items = append(items, retrieval.SourceItem{
						RefKind: domain.ChunkRefOutlineNode, RefID: node.ID,
						Text: formatOutlineNodeSource(outline, node),
					})
				}
			}
		}
	}
	return items, nil
}

// indexedRefKinds 返回全量重建时该作品会写哪些来源类型（用于剪枝）。
func indexedRefKinds(workKind string) []string {
	if workKind == domain.WorkKindCreative {
		return []string{
			domain.ChunkRefChapter, domain.ChunkRefCharacter,
			domain.ChunkRefWorldRule, domain.ChunkRefOutlineNode,
		}
	}
	return []string{domain.ChunkRefChapter}
}

// refIDsOf 取某来源类型下本次写入的全部 ref id（供剪枝保留）。
func refIDsOf(items []retrieval.SourceItem, refKind string) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		if item.RefKind == refKind && item.RefID != "" {
			ids = append(ids, item.RefID)
		}
	}
	return ids
}

// formatCharacterSource 把人物压成一段可检索文本（含 DNA 权重里非空的部分）。
func formatCharacterSource(c domain.CreativeCharacter) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "人物 %s（%s，重要度 %d）：%s", c.Name, c.SourceType, c.Importance, c.Description)
	dims := c.DNA.Dimensions()
	names := make([]string, 0, len(dims))
	for name := range dims {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		dim := dims[name]
		if dim.Weight <= 0 || strings.TrimSpace(dim.Text) == "" {
			continue
		}
		fmt.Fprintf(&sb, " %s(%d%%)=%s", name, dim.Weight, strings.TrimSpace(dim.Text))
	}
	return strings.TrimSpace(sb.String())
}

// formatWorldRuleSource 把世界规则压成一段可检索文本。
func formatWorldRuleSource(r domain.CreativeWorldRule) string {
	return fmt.Sprintf("世界规则 [%s][%s] %s：%s", r.Status, r.Category, r.Name, r.Description)
}

// formatOutlineNodeSource 把大纲节点压成一段可检索文本（章节链上的一个节点）。
func formatOutlineNodeSource(o domain.Outline, n domain.OutlineNode) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "大纲《%s》[第 %d 层] %s", o.Title, int(n.Level), n.Title)
	writeOutlinePart := func(label, value string) {
		if v := strings.TrimSpace(value); v != "" {
			fmt.Fprintf(&sb, "；%s：%s", label, v)
		}
	}
	writeOutlinePart("摘要", n.Summary)
	writeOutlinePart("目的", n.Purpose)
	writeOutlinePart("冲突", n.Conflict)
	writeOutlinePart("结果", n.Outcome)
	if len(n.Characters) > 0 {
		writeOutlinePart("出场", strings.Join(n.Characters, "、"))
	}
	writeOutlinePart("地点", n.Location)
	return sb.String()
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
// 支持二创的四类来源：章节正文、人物、世界规则、大纲节点。
// 其余组合（例如原著按章）退回全量重建：语义等价、只是多做一点活，
// 比"报错让任务失败"更符合"索引不该阻塞写作"的定位。
//
// 定位方式：章节走 GetChapter；设定类来源"列出该类型全部来源再按 ID 过滤"
// （人物/规则/大纲节点是几十条量级，为它们新增一批 by-id 仓储方法不划算）。
// 找不到 = 该来源已被删除 → 清空它的旧块（传空文本走 ReplaceChunks 的既有语义）。
func (s *IndexService) IndexRef(ctx context.Context, workKind, workID, refKind, refID string) (*retrieval.IndexResult, error) {
	if strings.TrimSpace(refID) == "" {
		return s.IndexWork(ctx, workKind, workID)
	}
	if workKind != domain.WorkKindCreative {
		fmt.Printf("[index] %s/%s 暂不支持按 ref 增量重建（ref=%s/%s），改为全量重建\n",
			workKind, workID, refKind, refID)
		return s.IndexWork(ctx, workKind, workID)
	}
	ownerID, err := s.users.DefaultUserID(ctx)
	if err != nil {
		return nil, err
	}
	switch refKind {
	case domain.ChunkRefChapter:
		return s.indexChapterRef(ctx, ownerID, workKind, workID, refID)
	case domain.ChunkRefCharacter, domain.ChunkRefWorldRule, domain.ChunkRefOutlineNode:
		items, err := s.collectCreative(ctx, workID)
		if err != nil {
			return nil, err
		}
		found := []retrieval.SourceItem{}
		for _, item := range items {
			if item.RefKind == refKind && item.RefID == refID {
				found = append(found, item)
			}
		}
		if len(found) == 0 {
			// 来源已被删除：清空它的旧块
			found = append(found, retrieval.SourceItem{RefKind: refKind, RefID: refID})
		}
		return retrieval.IndexWork(ctx, s.store, ownerID, workKind, workID, found)
	default:
		fmt.Printf("[index] %s/%s 未知来源类型 %s（ref_id=%s），改为全量重建\n",
			workKind, workID, refKind, refID)
		return s.IndexWork(ctx, workKind, workID)
	}
}

// indexChapterRef 重建单章索引（章节是唯一会被反复修改的来源：作者编辑、AI 写本章）。
func (s *IndexService) indexChapterRef(
	ctx context.Context, ownerID, workKind, workID, refID string,
) (*retrieval.IndexResult, error) {
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
