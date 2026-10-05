package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/yanaoyi/novamindv2/backend/internal/domain"
	"github.com/yanaoyi/novamindv2/backend/internal/repository"
)

// MemoryStore 是记忆仓储能力（由 repository.MemoryRepo 实现）。
type MemoryStore interface {
	CreateFacts(ctx context.Context, ownerUserID, workID string, facts []repository.FactWrite) (repository.FactWriteOutcome, error)
	ListFacts(ctx context.Context, workID string, onlyActive bool) ([]domain.MemoryFact, error)
	UpsertSummary(ctx context.Context, ownerUserID, chapterID, summary string) error
	GetSummary(ctx context.Context, chapterID string) (*domain.ChapterSummary, error)
}

// MemoryChapterReader 取章节（由 WritingRepo 实现）。
type MemoryChapterReader interface {
	GetChapter(ctx context.Context, id string) (*domain.CreativeChapter, error)
}

// MemoryIndexer 把"单条即一块"的来源写进检索索引（由 IndexService 实现）。
type MemoryIndexer interface {
	IndexText(ctx context.Context, workKind, workID, refKind, refID, text string, chapterID *string) error
}

// ConsistencyTrigger 把"抽取完成后自动查一次一致性"排成任务（由 TaskService 实现，§9.4）。
type ConsistencyTrigger interface {
	EnqueueConsistencyCheck(ctx context.Context, workID string, chapterIDs []string) error
}

// MemoryService 抽取并保存长篇记忆（Phase 9 §9.3.2）。
//
// 闭环：章节正文变化 → 异步抽取 → facts/summary 入库 → 写进检索索引
//
//	→ 自动触发一次一致性检查（检查器此时能看到 memory_facts）。
type MemoryService struct {
	repo      MemoryStore
	chapters  MemoryChapterReader
	ctxReader ChapterContextReader
	users     OwnerResolver
	indexer   MemoryIndexer
	checks    ConsistencyTrigger
}

// NewMemoryService 构建服务。
func NewMemoryService(
	repo MemoryStore,
	chapters MemoryChapterReader,
	ctxReader ChapterContextReader,
	users OwnerResolver,
	indexer MemoryIndexer,
) *MemoryService {
	return &MemoryService{repo: repo, chapters: chapters, ctxReader: ctxReader, users: users, indexer: indexer}
}

// SetConsistencyTrigger 注入"抽取完自动查一致性"的能力。
func (s *MemoryService) SetConsistencyTrigger(t ConsistencyTrigger) { s.checks = t }

// ExtractResult 是一次事实抽取的结果（任务输出与冒烟校验都用它）。
type ExtractResult struct {
	ChapterID       string `json:"chapter_id"`
	FactsCreated    int    `json:"facts_created"`
	FactsSuperseded int    `json:"facts_superseded"`
	Indexed         int    `json:"indexed"`
	SummaryChars    int    `json:"summary_chars"`
}

// ExtractFacts 从某章正文抽取事实与摘要并入库（供 extract_facts 任务调用）。
func (s *MemoryService) ExtractFacts(
	ctx context.Context,
	chapterID string,
	runner PromptRunner,
	report func(stage string, percent int),
) (*ExtractResult, error) {
	chapter, err := s.chapters.GetChapter(ctx, chapterID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(chapter.Content) == "" {
		return nil, fmt.Errorf("%w：本章还没有正文，没有可抽取的记忆", ErrBadRequest)
	}
	ownerID, err := s.users.DefaultUserID(ctx)
	if err != nil {
		return nil, err
	}
	if report != nil {
		report("准备上下文", 10)
	}
	characters, world := s.buildContext(ctx, chapter.CreativeWorkID)
	obj, err := RunJSONPromptValidated(ctx, runner, "fact_extract", "fact_extract",
		factExtractVars(chapter, characters, world), 2)
	if err != nil {
		return nil, err
	}
	if report != nil {
		report("写入记忆", 70)
	}

	chapterRef := chapter.ID
	writes := parseFactWrites(obj, &chapterRef)
	outcome, err := s.repo.CreateFacts(ctx, ownerID, chapter.CreativeWorkID, writes)
	if err != nil {
		return nil, err
	}
	summary := strings.TrimSpace(strVal(obj, "summary"))
	if summary != "" {
		if err := s.repo.UpsertSummary(ctx, ownerID, chapterID, summary); err != nil {
			return nil, err
		}
	}

	indexed := s.indexFacts(ctx, chapter, outcome, summary)

	// §9.3.2 的闭环：抽取完自动触发一次一致性检查（检查器这时才看得到新事实）。
	// 失败只记日志：记忆已经入库，检查晚一点不影响正确性。
	if s.checks != nil && (len(outcome.Created) > 0 || summary != "") {
		if err := s.checks.EnqueueConsistencyCheck(ctx, chapter.CreativeWorkID, []string{chapterID}); err != nil {
			fmt.Printf("[warn] 抽取后触发一致性检查失败（不影响记忆入库）: chapter=%s %v\n", chapterID, err)
		}
	}
	if report != nil {
		report("完成", 100)
	}
	return &ExtractResult{
		ChapterID:       chapterID,
		FactsCreated:    len(outcome.Created),
		FactsSuperseded: len(outcome.Superseded),
		Indexed:         indexed,
		SummaryChars:    len([]rune(summary)),
	}, nil
}

// buildContext 取抽取时用来对齐 subject 写法的设定（人物 + 世界规则）。
//
// 为什么不用一致性检查那套五类上下文：抽取只需要"人名与世界规则怎么写"，
// 塞进时间线与继承映射只会挤占正文的预算。
func (s *MemoryService) buildContext(ctx context.Context, workID string) (characters, world string) {
	if s.ctxReader == nil {
		return "", ""
	}
	if list, err := s.ctxReader.ListCharacters(ctx, workID); err == nil {
		var sb strings.Builder
		for i, c := range list {
			if i >= 40 {
				break
			}
			fmt.Fprintf(&sb, "- %s：%s\n", c.Name, strings.TrimSpace(c.Description))
		}
		characters = strings.TrimSpace(sb.String())
	}
	if detail, err := s.ctxReader.GetWorldDetail(ctx, workID); err == nil && detail.World != nil {
		var sb strings.Builder
		fmt.Fprintf(&sb, "世界：%s %s\n", detail.World.Name, detail.World.Description)
		for i, rule := range detail.Rules {
			if i >= 40 {
				break
			}
			if rule.Status == domain.RuleRemoved {
				continue
			}
			fmt.Fprintf(&sb, "- [%s] %s：%s\n", rule.Category, rule.Name, rule.Description)
		}
		world = strings.TrimSpace(sb.String())
	}
	return characters, world
}

// factExtractVars 构造 fact_extract 模板变量。
func factExtractVars(chapter *domain.CreativeChapter, characters, world string) map[string]any {
	return map[string]any{
		"ChapterNo":        chapter.ChapterNo,
		"ChapterTitle":     chapter.Title,
		"ChapterGoal":      strings.TrimSpace(chapter.Purpose + " " + chapter.Summary),
		"CharacterContext": characters,
		"WorldContext":     world,
		"ChapterText":      trimChars(chapter.Content, maxAnalysisChars),
	}
}

// parseFactWrites 把模型输出转成待写入的事实。
//
// 到这里形状已经由 JSON Schema 校验保证（kind/subject/fact 必填、kind 在枚举内），
// 所以这里只做直读；真正的合法性仍由仓储的 Validate 再兜一次。
func parseFactWrites(obj map[string]any, chapterID *string) []repository.FactWrite {
	items := asSlice(obj["facts"])
	out := make([]repository.FactWrite, 0, len(items))
	for _, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, repository.FactWrite{
			Kind:    strVal(item, "kind"),
			Subject: strVal(item, "subject"),
			Fact:    strVal(item, "fact"),
			// 来源章节：记下来才能回答"这条记忆是哪一章确立的"
			ChapterID: chapterID,
		})
	}
	return out
}

// indexFacts 把新事实与摘要写进检索索引，并把被替代事实的旧块清掉。
//
// 为什么要清：检索要的是"当前成立的事实"，一条已被替代的旧状态如果还留在索引里，
// 模型会同时看到"受伤"和"已痊愈"两条，凭空制造矛盾。
func (s *MemoryService) indexFacts(
	ctx context.Context,
	chapter *domain.CreativeChapter,
	outcome repository.FactWriteOutcome,
	summary string,
) int {
	if s.indexer == nil {
		return 0
	}
	indexed := 0
	for _, fact := range outcome.Created {
		text := fmt.Sprintf("[%s] %s：%s", fact.Kind, fact.Subject, fact.Fact)
		if err := s.indexer.IndexText(ctx, domain.WorkKindCreative, chapter.CreativeWorkID,
			domain.ChunkRefMemoryFact, fact.ID, text, &chapter.ID); err != nil {
			fmt.Printf("[warn] 记忆事实写索引失败（不影响入库）: fact=%s %v\n", fact.ID, err)
			continue
		}
		indexed++
	}
	for _, id := range outcome.Superseded {
		// 空文本 = 清空该来源的块（ReplaceChunks 的既有语义）
		if err := s.indexer.IndexText(ctx, domain.WorkKindCreative, chapter.CreativeWorkID,
			domain.ChunkRefMemoryFact, id, "", &chapter.ID); err != nil {
			fmt.Printf("[warn] 清理被替代事实的索引失败: fact=%s %v\n", id, err)
		}
	}
	if summary != "" {
		if err := s.indexer.IndexText(ctx, domain.WorkKindCreative, chapter.CreativeWorkID,
			domain.ChunkRefChapterSummary, chapter.ID, summary, &chapter.ID); err != nil {
			fmt.Printf("[warn] 章节摘要写索引失败（不影响入库）: chapter=%s %v\n", chapter.ID, err)
		} else {
			indexed++
		}
	}
	return indexed
}
