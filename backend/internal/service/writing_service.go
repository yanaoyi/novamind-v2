package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	// 内部上下文组装包与标准库 context 同名，这里按别名引入（ctxengine）。
	ctxengine "github.com/yanaoyi/novamindv2/backend/internal/context"
	"github.com/yanaoyi/novamindv2/backend/internal/domain"
	"github.com/yanaoyi/novamindv2/backend/internal/retrieval"
)

// WritingRepository 是写作系统需要的仓储能力。
type WritingRepository interface {
	CreateVolume(ctx context.Context, v *domain.CreativeVolume) error
	ListVolumes(ctx context.Context, workID string) ([]domain.CreativeVolume, error)

	CreateChapter(ctx context.Context, c *domain.CreativeChapter) error
	GetChapter(ctx context.Context, id string) (*domain.CreativeChapter, error)
	ListChapters(ctx context.Context, workID string, withContent bool) ([]domain.CreativeChapter, error)
	// 前情提要只需要最近几章，别把整本目录都查出来（审查 P2）
	ListChaptersBefore(ctx context.Context, workID string, chapterNo int, limit int) ([]domain.CreativeChapter, error)
	UpdateChapter(ctx context.Context, c *domain.CreativeChapter) error
	DeleteChapter(ctx context.Context, id string) error

	NextVersionNo(ctx context.Context, chapterID string) (int, error)
	CreateVersion(ctx context.Context, v *domain.ChapterVersion) error
	ListVersions(ctx context.Context, chapterID string) ([]domain.ChapterVersion, error)
	GetVersion(ctx context.Context, chapterID string, versionNo int) (*domain.ChapterVersion, error)

	CreateScene(ctx context.Context, s *domain.CreativeScene) error
	ListScenes(ctx context.Context, chapterID string) ([]domain.CreativeScene, error)

	CreateIssues(ctx context.Context, issues []domain.ConsistencyIssue) (int, error)
	ListIssues(ctx context.Context, workID, status string) ([]domain.ConsistencyIssue, error)
	UpdateIssueStatus(ctx context.Context, id, status string) error
}

// ChapterWriter 让 AI 写作任务能拿到"写成什么样"的上下文（二创人物/世界规则/前情）。
type ChapterContextReader interface {
	ListCharacters(ctx context.Context, workID string) ([]domain.CreativeCharacter, error)
	GetWorldDetail(ctx context.Context, workID string) (*WorldDetail, error)
	// 一致性检查（规格书 §39）还要：二创时间线 + 原著↔二创映射（查"原著继承一致性"）
	GetTimeline(ctx context.Context, workID string) ([]domain.CreativeTimelineEvent, error)
	ListMappings(ctx context.Context, workID string) ([]domain.OriginalCreativeMapping, error)
}

// WritingService 是写作系统服务。
type WritingService struct {
	repo      WritingRepository
	creative  CreativeWorkReader
	ctxReader ChapterContextReader
	// snapshots 是上下文快照记录器（Phase 9 §9.2.3）。用窄接口注入而不是直接依赖 SnapshotService，
	// 避免"SnapshotService 依赖 WritingService、WritingService 又依赖 SnapshotService"的循环。
	snapshots SnapshotRecorder
	// retriever 是上下文组装时的检索来源（Phase 9 §9.2 接线）。未注入时检索段为空，写作照常。
	retriever ContextRetriever
	// indexer 是"正文变了就重建这部分索引"的触发点（Phase 9 §9.1.4）。未注入时不留索引。
	indexer IndexTrigger
}

// SnapshotRecorder 是"记录一次 AI 调用前的上下文快照"的能力（由 SnapshotService 实现）。
type SnapshotRecorder interface {
	Record(ctx context.Context, chapterID *string, kind domain.SnapshotKind, snapshot map[string]any) (*domain.ContextSnapshot, error)
}

// SetSnapshotRecorder 注入快照记录器（可选：未注入时写作照常，只是不留快照）。
func (s *WritingService) SetSnapshotRecorder(r SnapshotRecorder) { s.snapshots = r }

// ContextRetriever 是"组装上下文时要用的检索"能力（由 RetrievalService 实现）。
//
// 同样用窄接口 + 可选注入：检索是增强能力，没接线时写作必须照常跑通
// （这与"快照失败不打断写作"是同一条纪律）。
type ContextRetriever interface {
	Search(ctx context.Context, workKind, workID, query string, topK int) ([]retrieval.ScoredChunk, error)
}

// SetRetriever 注入检索（Phase 9 §9.2 接线）。
func (s *WritingService) SetRetriever(r ContextRetriever) { s.retriever = r }

// SetIndexTrigger 注入索引入队能力（Phase 9 §9.1.4：正文保存/生成完成就重建该章索引）。
func (s *WritingService) SetIndexTrigger(t IndexTrigger) { s.indexer = t }

// triggerChapterIndex 入队重建本章索引；失败只记日志。
//
// 索引是"越写越懂"的供给端，但它绝不该成为写作链路的单点故障：
// 多写一章而索引晚几秒追平，是可接受的；因为索引失败而不让作者保存，不可接受。
func (s *WritingService) triggerChapterIndex(ctx context.Context, workID, chapterID string) {
	if s.indexer == nil {
		return
	}
	if err := s.indexer.EnqueueIndex(ctx, domain.WorkKindCreative, workID, domain.ChunkRefChapter, chapterID); err != nil {
		fmt.Printf("[warn] 章节索引入队失败（不影响保存）: chapter=%s %v\n", chapterID, err)
	}
}

// PromptMetaResolver 由 ModelInvoker 实现（可选）：在**调模型之前**报出
// "这次要用哪个模型、哪个模板版本"，供快照如实记录。
//
// 之前快照里写的是占位常量（"default" / 硬编码版本号），那是真信息缺口：
// 事后追溯"当时给了 AI 什么"时，模型与模板版本恰恰是回答"为什么这么写"的关键。
type PromptMetaResolver interface {
	DescribePrompt(ctx context.Context, promptName string) (model, promptVersion string)
}

// CreativeWorkReader 只需要"确认二创作品存在并拿到它"。
type CreativeWorkReader interface {
	GetWorkByID(ctx context.Context, id string) (*domain.CreativeWork, error)
}

// NewWritingService 构建服务。
func NewWritingService(repo WritingRepository, creative CreativeWorkReader, ctxReader ChapterContextReader) *WritingService {
	return &WritingService{repo: repo, creative: creative, ctxReader: ctxReader}
}

// CreateVolumeInput 是卷入参。
type CreateVolumeInput struct {
	Title    string
	Summary  string
	Sequence int
}

// CreateVolume 新增卷。
func (s *WritingService) CreateVolume(ctx context.Context, workID string, in CreateVolumeInput) (*domain.CreativeVolume, error) {
	if _, err := s.creative.GetWorkByID(ctx, workID); err != nil {
		return nil, err
	}
	v := &domain.CreativeVolume{CreativeWorkID: workID, Title: in.Title, Summary: in.Summary, Sequence: in.Sequence}
	v.Normalize()
	if err := v.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.CreateVolume(ctx, v); err != nil {
		return nil, err
	}
	return v, nil
}

// ListVolumes 列出卷。
func (s *WritingService) ListVolumes(ctx context.Context, workID string) ([]domain.CreativeVolume, error) {
	items, err := s.repo.ListVolumes(ctx, workID)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []domain.CreativeVolume{}
	}
	return items, nil
}

// CreateChapterInput 是章节入参（含大纲信息）。
type CreateChapterInput struct {
	VolumeID  *string
	ChapterNo int
	Title     string
	Summary   string
	Purpose   string
	Conflict  string
	Outcome   string
	Content   string
}

// CreateChapter 新增章节；正文非空时会同时生成 v1 版本。
func (s *WritingService) CreateChapter(ctx context.Context, workID string, in CreateChapterInput) (*domain.CreativeChapter, error) {
	if _, err := s.creative.GetWorkByID(ctx, workID); err != nil {
		return nil, err
	}
	c := &domain.CreativeChapter{
		CreativeWorkID: workID, VolumeID: in.VolumeID, ChapterNo: in.ChapterNo, Title: in.Title,
		Summary: in.Summary, Purpose: in.Purpose, Conflict: in.Conflict, Outcome: in.Outcome,
		Content: in.Content, Status: domain.ChapterDraft,
	}
	c.Normalize()
	if err := c.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.CreateChapter(ctx, c); err != nil {
		return nil, err
	}
	if strings.TrimSpace(c.Content) != "" {
		if err := s.snapshot(ctx, c, "创建章节"); err != nil {
			return nil, err
		}
		// 正文一落库就把索引追平（§9.1.4 触发点）：作者手写的第 1 章马上可以被后续章节检索到
		s.triggerChapterIndex(ctx, c.CreativeWorkID, c.ID)
	}
	return c, nil
}

// UpdateChapterInput 是章节更新入参。
type UpdateChapterInput struct {
	Title    *string
	Summary  *string
	Content  *string
	Status   *domain.ChapterStatus
	VolumeID *string
	Purpose  *string
	Conflict *string
	Outcome  *string
}

// UpdateChapter 更新章节；正文变化时会新建一个版本（规格书 §59）。
func (s *WritingService) UpdateChapter(ctx context.Context, id string, in UpdateChapterInput) (*domain.CreativeChapter, bool, error) {
	c, err := s.repo.GetChapter(ctx, id)
	if err != nil {
		return nil, false, err
	}
	contentChanged := false
	if in.Title != nil {
		c.Title = *in.Title
	}
	if in.Summary != nil {
		c.Summary = *in.Summary
	}
	if in.Status != nil {
		c.Status = *in.Status
	}
	if in.VolumeID != nil {
		c.VolumeID = in.VolumeID
	}
	if in.Purpose != nil {
		c.Purpose = *in.Purpose
	}
	if in.Conflict != nil {
		c.Conflict = *in.Conflict
	}
	if in.Outcome != nil {
		c.Outcome = *in.Outcome
	}
	if in.Content != nil && *in.Content != c.Content {
		c.Content = *in.Content
		contentChanged = true
	}
	c.Normalize()
	if err := c.Validate(); err != nil {
		return nil, false, err
	}
	if err := s.repo.UpdateChapter(ctx, c); err != nil {
		return nil, false, err
	}
	if contentChanged {
		if err := s.snapshot(ctx, c, "编辑正文"); err != nil {
			return nil, false, err
		}
		s.triggerChapterIndex(ctx, c.CreativeWorkID, c.ID)
	}
	return c, contentChanged, nil
}

func (s *WritingService) snapshot(ctx context.Context, c *domain.CreativeChapter, note string) error {
	no, err := s.repo.NextVersionNo(ctx, c.ID)
	if err != nil {
		return err
	}
	return s.repo.CreateVersion(ctx, &domain.ChapterVersion{
		ChapterID: c.ID, VersionNo: no, Content: c.Content, WordCount: c.WordCount, Note: note,
	})
}

// GetChapter 取章节。
func (s *WritingService) GetChapter(ctx context.Context, id string) (*domain.CreativeChapter, error) {
	return s.repo.GetChapter(ctx, id)
}

// ListChapters 列出章节。
func (s *WritingService) ListChapters(ctx context.Context, workID string, withContent bool) ([]domain.CreativeChapter, error) {
	if _, err := s.creative.GetWorkByID(ctx, workID); err != nil {
		return nil, err
	}
	items, err := s.repo.ListChapters(ctx, workID, withContent)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []domain.CreativeChapter{}
	}
	return items, nil
}

// DeleteChapter 删除章节。
func (s *WritingService) DeleteChapter(ctx context.Context, id string) error {
	// 先取一次拿作品归属：删掉之后就查不到它属于哪部作品了
	chapter, err := s.repo.GetChapter(ctx, id)
	if err != nil {
		return err
	}
	if err := s.repo.DeleteChapter(ctx, id); err != nil {
		return err
	}
	// 章节没了，它的索引块也必须消失（否则检索会召回已经不存在的正文）
	s.triggerChapterIndex(ctx, chapter.CreativeWorkID, id)
	return nil
}

// ListVersions 列出章节版本。
func (s *WritingService) ListVersions(ctx context.Context, chapterID string) ([]domain.ChapterVersion, error) {
	if _, err := s.repo.GetChapter(ctx, chapterID); err != nil {
		return nil, err
	}
	items, err := s.repo.ListVersions(ctx, chapterID)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []domain.ChapterVersion{}
	}
	return items, nil
}

// GetVersion 取某版本（含正文）。
func (s *WritingService) GetVersion(ctx context.Context, chapterID string, versionNo int) (*domain.ChapterVersion, error) {
	return s.repo.GetVersion(ctx, chapterID, versionNo)
}

// RestoreVersion 恢复到某个版本（当前正文会先存成新版本，避免丢内容）。
func (s *WritingService) RestoreVersion(ctx context.Context, chapterID string, versionNo int) (*domain.CreativeChapter, error) {
	version, err := s.repo.GetVersion(ctx, chapterID, versionNo)
	if err != nil {
		return nil, err
	}
	c, err := s.repo.GetChapter(ctx, chapterID)
	if err != nil {
		return nil, err
	}
	if c.Content != version.Content {
		// 先给"当前内容"留一版
		if err := s.snapshot(ctx, c, fmt.Sprintf("恢复 v%d 前的自动备份", versionNo)); err != nil {
			return nil, err
		}
	}
	c.Content = version.Content
	c.Normalize()
	if err := s.repo.UpdateChapter(ctx, c); err != nil {
		return nil, err
	}
	if err := s.snapshot(ctx, c, fmt.Sprintf("恢复自 v%d", versionNo)); err != nil {
		return nil, err
	}
	return c, nil
}

// CreateScene 新增场景。
func (s *WritingService) CreateScene(ctx context.Context, chapterID string, in domain.CreativeScene) (*domain.CreativeScene, error) {
	if _, err := s.repo.GetChapter(ctx, chapterID); err != nil {
		return nil, err
	}
	in.ChapterID = chapterID
	in.Normalize()
	if err := s.repo.CreateScene(ctx, &in); err != nil {
		return nil, err
	}
	return &in, nil
}

// ListScenes 列出场景。
func (s *WritingService) ListScenes(ctx context.Context, chapterID string) ([]domain.CreativeScene, error) {
	items, err := s.repo.ListScenes(ctx, chapterID)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []domain.CreativeScene{}
	}
	return items, nil
}

// ChapterContext 是喂给写作 Agent 的上下文（规格书 §31 的裁剪版）。
type ChapterContext struct {
	WorkTitle        string
	ChapterGoal      string
	Scene            string
	TimelineContext  string
	PreviousContext  string
	CharacterContext string
	WorldContext     string
}

// contextSources 是一次上下文取数的结果：组装出来的上下文 + 作品归属。
//
// 为什么要带归属：Phase 9 §9.2 的检索要按 work_id 查（原著一部、二创一部），
// 快照也要能标出"这条命中来自哪部作品"，光有 ChapterContext 是不够的。
type contextSources struct {
	Chapter        *domain.CreativeChapter
	Context        *ChapterContext
	CreativeWorkID string
	OriginalWorkID string
}

// maxTimelineEvents 限制进上下文的时间线条数（其余交给预算截断，别先做无用的字符串拼接）。
const maxTimelineEvents = 40

// BuildContext 组装写作上下文：人物 DNA + 世界规则 + 时间线 + 前几章摘要。
func (s *WritingService) BuildContext(ctx context.Context, chapterID string) (*ChapterContext, error) {
	src, err := s.loadContext(ctx, chapterID)
	if err != nil {
		return nil, err
	}
	return src.Context, nil
}

// loadContext 是取数入口：把"组装一次上下文需要的全部事实"一次取齐。
func (s *WritingService) loadContext(ctx context.Context, chapterID string) (*contextSources, error) {
	chapter, err := s.repo.GetChapter(ctx, chapterID)
	if err != nil {
		return nil, err
	}
	work, err := s.creative.GetWorkByID(ctx, chapter.CreativeWorkID)
	if err != nil {
		return nil, err
	}
	src := &contextSources{
		Chapter: chapter,
		Context: &ChapterContext{
			WorkTitle:   work.Title,
			ChapterGoal: strings.TrimSpace(chapter.Purpose + " " + chapter.Summary),
			Scene:       chapter.Title,
		},
		CreativeWorkID: chapter.CreativeWorkID,
		OriginalWorkID: work.OriginalWorkID,
	}
	out := src.Context

	if s.ctxReader != nil {
		if characters, err := s.ctxReader.ListCharacters(ctx, chapter.CreativeWorkID); err == nil {
			var sb strings.Builder
			for _, c := range characters {
				dims := []string{}
				for name, dim := range c.DNA.Dimensions() {
					if dim.Weight > 0 && dim.Text != "" {
						dims = append(dims, fmt.Sprintf("%s(%d%%)", name, dim.Weight))
					}
				}
				fmt.Fprintf(&sb, "- %s（%s）：%s %s\n", c.Name, c.SourceType, c.Description, strings.Join(dims, " "))
			}
			out.CharacterContext = sb.String()
		}
		if detail, err := s.ctxReader.GetWorldDetail(ctx, chapter.CreativeWorkID); err == nil && detail.World != nil {
			var sb strings.Builder
			fmt.Fprintf(&sb, "世界：%s %s\n", detail.World.Name, detail.World.Description)
			for _, r := range detail.Rules {
				if r.Status == domain.RuleRemoved {
					continue
				}
				fmt.Fprintf(&sb, "- [%s][%s] %s：%s\n", r.Status, r.Category, r.Name, r.Description)
			}
			out.WorldContext = sb.String()
		}
		// 时间线（§9.2.1 第 4 段）：按 sequence 排序，去掉已删除的事件。
		if events, err := s.ctxReader.GetTimeline(ctx, chapter.CreativeWorkID); err == nil {
			out.TimelineContext = formatTimeline(events)
		}
	}

	// 前情：本章之前最近 3 章的摘要
	if chapters, err := s.repo.ListChaptersBefore(ctx, chapter.CreativeWorkID, chapter.ChapterNo, 3); err == nil {
		previous := []string{}
		for _, c := range chapters {
			if c.Summary != "" {
				previous = append(previous, fmt.Sprintf("第%d章 %s：%s", c.ChapterNo, c.Title, c.Summary))
			}
		}
		out.PreviousContext = strings.Join(previous, "\n")
	}
	return src, nil
}

// formatTimeline 把二创时间线压成提示词用的一段文本。
func formatTimeline(events []domain.CreativeTimelineEvent) string {
	if len(events) == 0 {
		return ""
	}
	ordered := make([]domain.CreativeTimelineEvent, len(events))
	copy(ordered, events)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Sequence < ordered[j].Sequence })

	var sb strings.Builder
	kept := 0
	for _, e := range ordered {
		if kept >= maxTimelineEvents {
			break
		}
		if e.Status == domain.TimelineRemoved {
			continue
		}
		label := strings.TrimSpace(e.TimeLabel)
		if label == "" {
			label = fmt.Sprintf("第 %d 位", e.Sequence)
		}
		fmt.Fprintf(&sb, "- [%s] %s：%s %s\n", e.Status, label, e.Title, e.Description)
		kept++
	}
	return strings.TrimSpace(sb.String())
}

// retrievalQuery 由"本章要写什么 + 作者指令"派生检索词。
//
// 用章节目标而不是整段提示词：BM25 是词面匹配，塞太多无关词只会稀释权重；
// 截到 200 字也是同一个理由（过长的 query 每个 char-bigram 都会被算进去）。
func retrievalQuery(goal, scene, extra string) string {
	q := strings.TrimSpace(strings.Join([]string{goal, scene, extra}, " "))
	return trimChars(q, 200)
}

// gatherHits 按作品检索并转成上下文用的命中。
//
// 检索失败只记日志、返回空：检索是增强能力，不该成为写作链路的单点故障。
func (s *WritingService) gatherHits(
	ctx context.Context,
	src *contextSources,
	query string,
	topK int,
) (original, creative []ctxengine.RetrievalHit) {
	if s.retriever == nil || src == nil || strings.TrimSpace(query) == "" {
		return nil, nil
	}
	if src.OriginalWorkID != "" {
		original = s.searchHits(ctx, domain.WorkKindOriginal, src.OriginalWorkID, query, topK)
	}
	if src.CreativeWorkID != "" {
		creative = s.searchHits(ctx, domain.WorkKindCreative, src.CreativeWorkID, query, topK)
	}
	return original, creative
}

// searchHits 走一路检索并把结果转成组装层能用的结构。
func (s *WritingService) searchHits(ctx context.Context, workKind, workID, query string, topK int) []ctxengine.RetrievalHit {
	scored, err := s.retriever.Search(ctx, workKind, workID, query, topK)
	if err != nil {
		fmt.Printf("[warn] 检索失败（不影响本次写作）: %s/%s %v\n", workKind, workID, err)
		return nil
	}
	hits := make([]ctxengine.RetrievalHit, 0, len(scored))
	for _, c := range scored {
		hits = append(hits, ctxengine.RetrievalHit{
			ChunkID: c.ID, RefKind: c.RefKind, RefID: c.RefID, Seq: c.Seq,
			Content: c.Content, Score: c.Score,
		})
	}
	return hits
}

// chapterFacts 把已取到的事实装进组装层的入参。
func chapterFacts(chapterID string, c *ChapterContext) ctxengine.ChapterFacts {
	return ctxengine.ChapterFacts{
		ChapterID:        chapterID,
		ChapterGoal:      c.ChapterGoal,
		CharacterContext: c.CharacterContext,
		WorldContext:     c.WorldContext,
		TimelineContext:  c.TimelineContext,
		PreviousContext:  c.PreviousContext,
	}
}

// promptMeta 取"这次调用将使用的模型与模板版本"；解析不出来时如实写 unknown，不编造。
func promptMeta(ctx context.Context, runner PromptRunner, name string) (model, promptVersion string) {
	if r, ok := runner.(PromptMetaResolver); ok {
		m, v := r.DescribePrompt(ctx, name)
		if strings.TrimSpace(m) != "" && strings.TrimSpace(v) != "" {
			return m, name + "." + v
		}
	}
	return "unknown", "unknown"
}

// GenerateChapterDraft 用 AI 生成本章正文草稿（供写作任务调用）。
func (s *WritingService) GenerateChapterDraft(
	ctx context.Context,
	chapterID string,
	runner PromptRunner,
	targetWords int,
	instruction string,
	report func(stage string, percent int),
) (string, error) {
	src, err := s.loadContext(ctx, chapterID)
	if err != nil {
		return "", err
	}
	chapterCtx := src.Context
	if targetWords <= 0 {
		targetWords = 2000
	}
	if report != nil {
		report("组装上下文", 20)
	}
	// Phase 9 §9.2 接线：检索 → 8 段组装（预算内）→ 提示词。提示词里给模型看的东西
	// 与快照里记下的东西必须是同一份（下面 Snapshot 直接用 asm 的内容），否则
	// "追溯当时给了 AI 什么"就会退化成"猜当时大概给了什么"。
	originalHits, creativeHits := s.gatherHits(ctx, src,
		retrievalQuery(chapterCtx.ChapterGoal, chapterCtx.Scene, instruction), retrieval.DefaultTopK)
	asm := ctxengine.AssembleForChapter(
		chapterFacts(chapterID, chapterCtx), originalHits, creativeHits, instruction)
	// 调模型之前先落快照（Phase 9 §9.2.3）：回答"AI 当时为什么这么写"。
	// 失败只记日志、不打断写作 —— 快照是审计能力，不该成为写作链路的单点故障。
	s.recordGenerateSnapshot(ctx, chapterID, src, asm, originalHits, creativeHits, targetWords, runner)
	// 写本章要的是小说正文，走文本模式（JSON 模式会让上游返回空内容，见 ModelInvoker.RunTextPrompt）
	reply, err := runner.RunTextPrompt(ctx, "chapter_generate", chapterGenerateVars(asm, chapterCtx.Scene, targetWords))
	if err != nil {
		return "", err
	}
	if report != nil {
		report("写入草稿", 80)
	}
	draft := strings.TrimSpace(reply)
	if draft == "" {
		// 模型返回空内容时绝不能当成「生成成功」写进章节：那会静默产出 0 字正文，
		// 后续一致性检查只会报「没有可检查的正文」，使用者完全看不出发生了什么。
		return "", fmt.Errorf("模型没有返回正文（可能是上下文过长、被截断或触发内容过滤），请重试或精简设定")
	}
	return draft, nil
}

// RewriteAction 是编辑器内 AI 操作的类型。
type RewriteAction string

const (
	RewriteActionRewrite  RewriteAction = "改写"
	RewriteActionExpand   RewriteAction = "扩写"
	RewriteActionShorten  RewriteAction = "缩写"
	RewriteActionPolish   RewriteAction = "润色"
	RewriteActionConflict RewriteAction = "增强冲突"
	RewriteActionEmotion  RewriteAction = "增强情绪"
	// 规格书 §38 / §48 要求、此前缺失的五种操作
	RewriteActionContinue RewriteAction = "续写"
	RewriteActionAddAct   RewriteAction = "增加动作"
	RewriteActionAddTalk  RewriteAction = "增加对白"
	RewriteActionPacing   RewriteAction = "调整节奏"
	RewriteActionPOV      RewriteAction = "改变叙事视角"
)

// Valid 判断操作类型是否合法。
func (a RewriteAction) Valid() bool {
	switch a {
	case RewriteActionRewrite, RewriteActionExpand, RewriteActionShorten,
		RewriteActionPolish, RewriteActionConflict, RewriteActionEmotion,
		RewriteActionContinue, RewriteActionAddAct, RewriteActionAddTalk,
		RewriteActionPacing, RewriteActionPOV:
		return true
	default:
		return false
	}
}

// RewriteActions 返回全部支持的编辑器 AI 操作（前端与文档都以此为准）。
func RewriteActions() []string {
	return []string{
		string(RewriteActionRewrite), string(RewriteActionExpand), string(RewriteActionShorten),
		string(RewriteActionPolish), string(RewriteActionConflict), string(RewriteActionEmotion),
		string(RewriteActionContinue), string(RewriteActionAddAct), string(RewriteActionAddTalk),
		string(RewriteActionPacing), string(RewriteActionPOV),
	}
}

// RewriteInput 是编辑器 AI 操作入参。
type RewriteInput struct {
	ChapterID   string
	Text        string
	Action      RewriteAction
	Instruction string
}

// RewriteText 对选中文本做 AI 处理（同步返回，规格书 §38）。
func (s *WritingService) RewriteText(ctx context.Context, runner PromptRunner, in RewriteInput) (string, error) {
	if !in.Action.Valid() {
		return "", fmt.Errorf("%w：不支持的 AI 操作 %s", ErrBadRequest, in.Action)
	}
	if strings.TrimSpace(in.Text) == "" {
		return "", fmt.Errorf("%w：请先选中要处理的文本", ErrBadRequest)
	}
	src, err := s.loadContext(ctx, in.ChapterID)
	if err != nil {
		return "", err
	}
	chapterCtx := src.Context
	// §9.2 接线：编辑器内操作同样走"检索 → 8 段组装 → 提示词"（修订清单要求改写/扩写也覆盖）。
	// 检索词用"选中文本 + 作者补充要求"：改写要呼应的是与这段文字相关的既有内容。
	originalHits, creativeHits := s.gatherHits(ctx, src,
		retrievalQuery(chapterCtx.ChapterGoal, in.Text, in.Instruction), retrieval.DefaultTopK)
	asm := ctxengine.AssembleForChapter(
		chapterFacts(in.ChapterID, chapterCtx), originalHits, creativeHits, in.Instruction)
	// 改写/扩写/续写等编辑器内操作同样落快照（Phase 9 §9.2.3 要求覆盖这些链路）。
	// 失败只记日志、不影响这次改写（与写本章一致）。
	s.recordRewriteSnapshot(ctx, in, asm, originalHits, creativeHits, runner)
	// 改写/扩写/缩写等返回的也是正文
	return runner.RunTextPrompt(ctx, "rewrite", rewriteVars(asm, string(in.Action), in.Text))
}

// CheckConsistency 对指定章节做一致性检查，结果写入问题列表（规格书 §39）。
func (s *WritingService) CheckConsistency(
	ctx context.Context,
	workID string,
	chapterIDs []string,
	runner PromptRunner,
	report func(stage string, percent int),
) (ConsistencyResult, error) {
	work, err := s.creative.GetWorkByID(ctx, workID)
	if err != nil {
		return ConsistencyResult{}, err
	}
	chapters := make([]domain.CreativeChapter, 0)
	if len(chapterIDs) > 0 {
		for _, id := range chapterIDs {
			c, err := s.repo.GetChapter(ctx, id)
			if err != nil {
				return ConsistencyResult{}, err
			}
			if c.CreativeWorkID != workID {
				return ConsistencyResult{}, errors.New("章节不属于该二创作品")
			}
			chapters = append(chapters, *c)
		}
	} else {
		all, err := s.repo.ListChapters(ctx, workID, true)
		if err != nil {
			return ConsistencyResult{}, err
		}
		for _, c := range all {
			if strings.TrimSpace(c.Content) != "" {
				chapters = append(chapters, c)
			}
		}
	}
	if len(chapters) == 0 {
		return ConsistencyResult{}, errors.New("没有可检查的正文（章节还是空的）")
	}

	cctx := s.BuildConsistencyContext(ctx, workID)
	src := &contextSources{CreativeWorkID: work.ID, OriginalWorkID: work.OriginalWorkID}

	result := ConsistencyResult{}
	for i, chapter := range chapters {
		if report != nil {
			report(fmt.Sprintf("检查第 %d/%d 章", i+1, len(chapters)), 10+int(float64(i)/float64(len(chapters))*80))
		}
		result.Checked++

		// §9.4：一致性检查也走 Context Engine（kind=consistency，同样落快照）。
		// 检索词用被检查的正文本身 —— 要查的就是"这段文字与既有内容有没有冲突"。
		originalHits, creativeHits := s.gatherHits(ctx, src,
			retrievalQuery(chapter.Title, chapter.Content, cctx.Plot), retrieval.DefaultTopK)
		asm := ctxengine.AssembleForChapter(ctxengine.ChapterFacts{
			ChapterID:        chapter.ID,
			CharacterContext: cctx.Characters,
			WorldContext:     cctx.World,
			TimelineContext:  cctx.Timeline,
			PreviousContext:  cctx.Plot,
		}, originalHits, creativeHits, "")
		// 快照按"每一次 AI 调用"落一条（这里是每章一条，chapter_id 直接锚到该章），
		// 这样事后能精确回答"审这一章时给了模型什么"。
		s.recordConsistencySnapshot(ctx, workID, cctx, asm, originalHits, creativeHits, chapter, runner)

		// 规格书 §57/§58：模型输出必须结构化；失败要重试，重试仍失败要如实记录。
		// 以前是直接 continue —— 跳过等于"这章没问题"，是假阴性。
		obj, err := runConsistencyPrompt(ctx, runner, consistencyVars(asm, cctx.Inheritance, chapter.Content))
		if err != nil {
			result.FailedChapters = append(result.FailedChapters,
				fmt.Sprintf("第%d章 %s", chapter.ChapterNo, chapter.Title))
			continue
		}
		chapterID := chapter.ID
		issues := make([]domain.ConsistencyIssue, 0)
		for _, raw := range asSlice(obj["issues"]) {
			item, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			issues = append(issues, domain.ConsistencyIssue{
				CreativeWorkID: workID, ChapterID: &chapterID,
				Severity: strVal(item, "severity"), Type: strVal(item, "type"),
				Description: strVal(item, "description"), Evidence: strVal(item, "evidence"),
				Suggestion: strVal(item, "suggestion"),
			})
		}
		created, err := s.repo.CreateIssues(ctx, issues)
		if err != nil {
			return result, err
		}
		result.Created += created
	}
	if report != nil {
		report("完成", 100)
	}
	return result, nil
}

// ConsistencyResult 是一次一致性检查的结果（规格书 §58：失败必须可见，不能静默吞掉）。
type ConsistencyResult struct {
	Checked        int      `json:"checked"`
	Created        int      `json:"issues_created"`
	FailedChapters []string `json:"failed_chapters"`
}

// runConsistencyPrompt 送审单章（vars 由 consistencyVars 从组装结果构造）；
// 模型输出不是合法 JSON 时重试一次，两次都不行就返回错误，由调用方记为「本章检查失败」。
func runConsistencyPrompt(
	ctx context.Context,
	runner PromptRunner,
	vars map[string]any,
) (map[string]any, error) {
	var lastErr error
	for attempt := 1; attempt <= 2; attempt++ {
		reply, err := runner.RunPrompt(ctx, "consistency_check", vars)
		if err != nil {
			return nil, err
		}
		obj, err := ExtractJSONObject(reply)
		if err == nil {
			return obj, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("模型输出两次都不是合法 JSON：%w", lastErr)
}

// ListIssues 列出一致性问题。
func (s *WritingService) ListIssues(ctx context.Context, workID, status string) ([]domain.ConsistencyIssue, error) {
	if _, err := s.creative.GetWorkByID(ctx, workID); err != nil {
		return nil, err
	}
	items, err := s.repo.ListIssues(ctx, workID, status)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []domain.ConsistencyIssue{}
	}
	return items, nil
}

// UpdateIssueStatus 更新问题状态（RESOLVED / IGNORED）。
func (s *WritingService) UpdateIssueStatus(ctx context.Context, id, status string) error {
	if status != "OPEN" && status != "RESOLVED" && status != "IGNORED" {
		return domain.ErrIssueStatusInvalid
	}
	return s.repo.UpdateIssueStatus(ctx, id, status)
}

// Export 导出作品（TXT / Markdown / DOCX，规格书 §61）。
func (s *WritingService) Export(ctx context.Context, workID, format string) ([]byte, string, string, error) {
	work, err := s.creative.GetWorkByID(ctx, workID)
	if err != nil {
		return nil, "", "", err
	}
	volumes, err := s.repo.ListVolumes(ctx, workID)
	if err != nil {
		return nil, "", "", err
	}
	chapters, err := s.repo.ListChapters(ctx, workID, true)
	if err != nil {
		return nil, "", "", err
	}
	if len(chapters) == 0 {
		return nil, "", "", errors.New("还没有章节正文可导出")
	}

	volumeTitle := map[string]string{}
	for _, v := range volumes {
		volumeTitle[v.ID] = v.Title
	}

	switch strings.ToLower(format) {
	case "", "txt":
		var buf bytes.Buffer
		fmt.Fprintf(&buf, "%s\n\n", work.Title)
		if work.Description != "" {
			fmt.Fprintf(&buf, "（%s）\n\n", work.Description)
		}
		currentVolume := ""
		for _, c := range chapters {
			if c.VolumeID != nil {
				if title := volumeTitle[*c.VolumeID]; title != "" && title != currentVolume {
					currentVolume = title
					fmt.Fprintf(&buf, "【%s】\n\n", title)
				}
			}
			fmt.Fprintf(&buf, "第 %d 章 %s\n\n%s\n\n", c.ChapterNo, c.Title, c.Content)
		}
		return buf.Bytes(), work.Title + ".txt", "text/plain; charset=utf-8", nil

	case "md", "markdown":
		var buf bytes.Buffer
		fmt.Fprintf(&buf, "# %s\n\n", work.Title)
		if work.Description != "" {
			fmt.Fprintf(&buf, "> %s\n\n", work.Description)
		}
		currentVolume := ""
		for _, c := range chapters {
			if c.VolumeID != nil {
				if title := volumeTitle[*c.VolumeID]; title != "" && title != currentVolume {
					currentVolume = title
					fmt.Fprintf(&buf, "## %s\n\n", title)
				}
			}
			fmt.Fprintf(&buf, "### 第 %d 章 %s\n\n%s\n\n", c.ChapterNo, c.Title, c.Content)
		}
		return buf.Bytes(), work.Title + ".md", "text/markdown; charset=utf-8", nil

	case "docx":
		data, err := buildDocx(work.Title, chapters)
		if err != nil {
			return nil, "", "", err
		}
		return data, work.Title + ".docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", nil

	default:
		return nil, "", "", domain.ErrExportFormatInvalid
	}
}

// recordGenerateSnapshot 组装并写入一次"写本章"的上下文快照。
//
// payload 字段对齐修订清单 P0-3：模型 / Prompt 版本 / 作者指令 / 各段上下文 / 检索来源 /
// token 预算（created_at 由仓储写入，作品归属由 SnapshotService 从 chapter_id 反查）。
func (s *WritingService) recordGenerateSnapshot(
	ctx context.Context,
	chapterID string,
	src *contextSources,
	asm *ctxengine.Assembly,
	originalHits, creativeHits []ctxengine.RetrievalHit,
	targetWords int,
	runner PromptRunner,
) {
	if s.snapshots == nil || src == nil || src.Context == nil || asm == nil {
		return
	}
	model, promptVersion := promptMeta(ctx, runner, "chapter_generate")
	payload := map[string]any{
		"creative_work_id":   src.CreativeWorkID,
		"model":              model,
		"prompt_version":     promptVersion,
		"author_instruction": asm.Sections.AuthorInstruction,
		"target_words":       targetWords,
		"outline_context": map[string]any{
			"chapter_goal": asm.Sections.ChapterGoal,
			"scene":        src.Context.Scene,
		},
		"character_context":  asm.Sections.Characters,
		"world_context":      asm.Sections.World,
		"timeline_context":   asm.Sections.Timeline,
		"prev_summary":       asm.Sections.PrevSummary,
		"retrieved_original": asm.Sections.RetrievedOriginal,
		"retrieved_creative": asm.Sections.RetrievedCreative,
		"retrieved_sources":  snapshotSources(originalHits, creativeHits),
		"token_budget":       snapshotBudget(asm),
	}
	if src.OriginalWorkID != "" {
		payload["original_work_id"] = src.OriginalWorkID
	}
	if _, err := s.snapshots.Record(ctx, &chapterID, domain.SnapshotGenerate, payload); err != nil {
		fmt.Printf("[warn] 写本章快照失败（不影响本次生成）: %v\n", err)
	}
}

// snapshotKindForAction 把编辑器动作映射到快照 kind（清单定义的类型有限，其余归入 rewrite）。
func snapshotKindForAction(action RewriteAction) domain.SnapshotKind {
	switch action {
	case RewriteActionContinue:
		return domain.SnapshotContinue
	case RewriteActionExpand:
		return domain.SnapshotExpand
	default:
		return domain.SnapshotRewrite
	}
}

// recordRewriteSnapshot 记录一次编辑器内 AI 操作（改写/扩写/续写/缩写/润色…）的上下文快照。
func (s *WritingService) recordRewriteSnapshot(
	ctx context.Context,
	in RewriteInput,
	asm *ctxengine.Assembly,
	originalHits, creativeHits []ctxengine.RetrievalHit,
	runner PromptRunner,
) {
	if s.snapshots == nil || in.ChapterID == "" || asm == nil {
		return
	}
	model, promptVersion := promptMeta(ctx, runner, "rewrite")
	payload := map[string]any{
		"model":              model,
		"prompt_version":     promptVersion,
		"author_instruction": asm.Sections.AuthorInstruction,
		"action":             string(in.Action),
		"input_chars":        len([]rune(in.Text)),
		"character_context":  asm.Sections.Characters,
		"world_context":      asm.Sections.World,
		"retrieved_creative": asm.Sections.RetrievedCreative,
		"retrieved_sources":  snapshotSources(originalHits, creativeHits),
		"token_budget":       snapshotBudget(asm),
	}
	chapterID := in.ChapterID
	if _, err := s.snapshots.Record(ctx, &chapterID, snapshotKindForAction(in.Action), payload); err != nil {
		fmt.Printf("[warn] 编辑器 AI 操作快照失败（不影响本次处理）: %v\n", err)
	}
}

// recordConsistencySnapshot 记录一次一致性检查的上下文快照。
//
// 粒度是"每次 AI 调用"（即每章一条，chapter_id 锚到该章），而不是整批一条：
// 只有锚到章节，事后才能回答"审这一章时到底给了模型什么"。
func (s *WritingService) recordConsistencySnapshot(
	ctx context.Context,
	workID string,
	cctx ConsistencyContext,
	asm *ctxengine.Assembly,
	originalHits, creativeHits []ctxengine.RetrievalHit,
	chapter domain.CreativeChapter,
	runner PromptRunner,
) {
	if s.snapshots == nil || workID == "" || asm == nil {
		return
	}
	model, promptVersion := promptMeta(ctx, runner, "consistency_check")
	payload := map[string]any{
		"creative_work_id":    workID,
		"chapter_no":          chapter.ChapterNo,
		"chapter_chars":       len([]rune(chapter.Content)),
		"model":               model,
		"prompt_version":      promptVersion,
		"character_context":   asm.Sections.Characters,
		"world_context":       asm.Sections.World,
		"timeline_context":    asm.Sections.Timeline,
		"plot_context":        asm.Sections.PrevSummary,
		"inheritance_context": cctx.Inheritance,
		"retrieved_original":  asm.Sections.RetrievedOriginal,
		"retrieved_creative":  asm.Sections.RetrievedCreative,
		"retrieved_sources":   snapshotSources(originalHits, creativeHits),
		"token_budget":        snapshotBudget(asm),
	}
	chapterID := chapter.ID
	if _, err := s.snapshots.Record(ctx, &chapterID, domain.SnapshotConsistency, payload); err != nil {
		fmt.Printf("[warn] 一致性检查快照失败（不影响本次检查）: %v\n", err)
	}
}

// snapshotBudget 把"预算 / 用量 / 每段 token / 被截断的段"整理进快照。
func snapshotBudget(asm *ctxengine.Assembly) map[string]any {
	truncated := asm.Truncated
	if truncated == nil {
		truncated = []string{}
	}
	return map[string]any{
		"limit":      ctxengine.TotalBudget,
		"used":       asm.Total,
		"by_section": asm.Tokens,
		"truncated":  truncated,
	}
}

// snapshotSources 合并两条检索路的来源清单（原著 + 二创），每条都带 work_kind 以便区分。
func snapshotSources(originalHits, creativeHits []ctxengine.RetrievalHit) []map[string]any {
	out := ctxengine.RetrievalSources(domain.WorkKindOriginal, originalHits)
	return append(out, ctxengine.RetrievalSources(domain.WorkKindCreative, creativeHits)...)
}
