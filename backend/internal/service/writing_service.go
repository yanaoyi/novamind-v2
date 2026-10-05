package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/yanaoyi/novamindv2/backend/internal/domain"
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
}

// SnapshotRecorder 是"记录一次 AI 调用前的上下文快照"的能力（由 SnapshotService 实现）。
type SnapshotRecorder interface {
	Record(ctx context.Context, chapterID *string, kind domain.SnapshotKind, snapshot map[string]any) (*domain.ContextSnapshot, error)
}

// SetSnapshotRecorder 注入快照记录器（可选：未注入时写作照常，只是不留快照）。
func (s *WritingService) SetSnapshotRecorder(r SnapshotRecorder) { s.snapshots = r }

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
	return s.repo.DeleteChapter(ctx, id)
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
	PreviousContext  string
	CharacterContext string
	WorldContext     string
}

// BuildContext 组装写作上下文：人物 DNA + 世界规则 + 前几章摘要。
func (s *WritingService) BuildContext(ctx context.Context, chapterID string) (*ChapterContext, error) {
	chapter, err := s.repo.GetChapter(ctx, chapterID)
	if err != nil {
		return nil, err
	}
	work, err := s.creative.GetWorkByID(ctx, chapter.CreativeWorkID)
	if err != nil {
		return nil, err
	}
	out := &ChapterContext{
		WorkTitle:   work.Title,
		ChapterGoal: strings.TrimSpace(chapter.Purpose + " " + chapter.Summary),
		Scene:       chapter.Title,
	}

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
	return out, nil
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
	chapterCtx, err := s.BuildContext(ctx, chapterID)
	if err != nil {
		return "", err
	}
	if targetWords <= 0 {
		targetWords = 2000
	}
	if report != nil {
		report("组装上下文", 20)
	}
	// 调模型之前先落快照（Phase 9 §9.2.3）：回答"AI 当时为什么这么写"。
	// 失败只记日志、不打断写作 —— 快照是审计能力，不该成为写作链路的单点故障。
	s.recordGenerateSnapshot(ctx, chapterID, chapterCtx, targetWords, instruction)
	// 写本章要的是小说正文，走文本模式（JSON 模式会让上游返回空内容，见 ModelInvoker.RunTextPrompt）
	reply, err := runner.RunTextPrompt(ctx, "chapter_generate", map[string]any{
		"TargetWords":      targetWords,
		"ChapterGoal":      chapterCtx.ChapterGoal,
		"Scene":            chapterCtx.Scene,
		"CharacterContext": chapterCtx.CharacterContext,
		"WorldContext":     chapterCtx.WorldContext,
		"PreviousContext":  chapterCtx.PreviousContext,
		"Instruction":      instruction,
		// v2 模板新增的两段检索内容。模板引擎的 missingkey=error 要求这两段必须传，
		// 否则渲染直接失败（引入 v2 时必须同步补上，别让"模板先行、调用方后补"变成静默故障）。
		// 检索接线（§9.2）拿到真实命中后，把这两项换成 FormatRetrieval 的结果即可。
		"RetrievedOriginal": "",
		"RetrievedCreative": "",
	})
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
	// 改写/扩写/续写等编辑器内操作同样落快照（Phase 9 §9.2.3 要求覆盖这些链路）。
	// 失败只记日志、不影响这次改写（与写本章一致）。
	s.recordRewriteSnapshot(ctx, in)
	if strings.TrimSpace(in.Text) == "" {
		return "", fmt.Errorf("%w：请先选中要处理的文本", ErrBadRequest)
	}
	chapterCtx, err := s.BuildContext(ctx, in.ChapterID)
	if err != nil {
		return "", err
	}
	// 改写/扩写/缩写等返回的也是正文
	return runner.RunTextPrompt(ctx, "rewrite", map[string]any{
		"Action":           string(in.Action),
		"Text":             in.Text,
		"Instruction":      in.Instruction,
		"CharacterContext": chapterCtx.CharacterContext,
		"WorldContext":     chapterCtx.WorldContext,
	})
}

// CheckConsistency 对指定章节做一致性检查，结果写入问题列表（规格书 §39）。
func (s *WritingService) CheckConsistency(
	ctx context.Context,
	workID string,
	chapterIDs []string,
	runner PromptRunner,
	report func(stage string, percent int),
) (ConsistencyResult, error) {
	if _, err := s.creative.GetWorkByID(ctx, workID); err != nil {
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

	result := ConsistencyResult{}
	for i, chapter := range chapters {
		if report != nil {
			report(fmt.Sprintf("检查第 %d/%d 章", i+1, len(chapters)), 10+int(float64(i)/float64(len(chapters))*80))
		}
		result.Checked++

		// 规格书 §57/§58：模型输出必须结构化；失败要重试，重试仍失败要如实记录。
		// 以前是直接 continue —— 跳过等于"这章没问题"，是假阴性。
		obj, err := runConsistencyPrompt(ctx, runner, cctx, chapter.Content)
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

// runConsistencyPrompt 送审单章；模型输出不是合法 JSON 时重试一次，
// 两次都不行就返回错误，由调用方记为「本章检查失败」。
func runConsistencyPrompt(
	ctx context.Context,
	runner PromptRunner,
	cctx ConsistencyContext,
	chapterContent string,
) (map[string]any, error) {
	vars := map[string]any{
		"CharacterContext":   cctx.Characters,
		"WorldContext":       cctx.World,
		"TimelineContext":    cctx.Timeline,
		"PlotContext":        cctx.Plot,
		"InheritanceContext": cctx.Inheritance,
		"ChapterText":        trimChars(chapterContent, maxAnalysisChars),
	}

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
	chapterCtx *ChapterContext,
	targetWords int,
	instruction string,
) {
	if s.snapshots == nil || chapterCtx == nil {
		return
	}
	payload := map[string]any{
		"model_provider":     "default",
		"prompt_version":     "chapter_generate.v2",
		"author_instruction": instruction,
		"target_words":       targetWords,
		"outline_context": map[string]any{
			"chapter_goal": chapterCtx.ChapterGoal,
			"scene":        chapterCtx.Scene,
		},
		"character_context": chapterCtx.CharacterContext,
		"world_context":     chapterCtx.WorldContext,
		"prev_summary":      chapterCtx.PreviousContext,
		// 检索来源在 §9.1 接线完成后填充；当前空数组，不放假数据
		"retrieved_sources": []any{},
		"token_budget": map[string]any{
			"limit": 8000,
		},
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
func (s *WritingService) recordRewriteSnapshot(ctx context.Context, in RewriteInput) {
	if s.snapshots == nil || in.ChapterID == "" {
		return
	}
	payload := map[string]any{
		"model_provider":     "default",
		"prompt_version":     "rewrite.v2",
		"author_instruction": in.Instruction,
		"action":             string(in.Action),
		"input_chars":        len([]rune(in.Text)),
		"retrieved_sources":  []any{},
		"token_budget": map[string]any{
			"limit": 8000,
		},
	}
	chapterID := in.ChapterID
	if _, err := s.snapshots.Record(ctx, &chapterID, snapshotKindForAction(in.Action), payload); err != nil {
		fmt.Printf("[warn] 编辑器 AI 操作快照失败（不影响本次处理）: %v\n", err)
	}
}
