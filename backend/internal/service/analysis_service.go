package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/yanaoyi/novamindv2/backend/internal/domain"
	"github.com/yanaoyi/novamindv2/backend/internal/repository"
)

// AnalysisProposalRepository 是分析服务需要的提案仓储能力。
type AnalysisProposalRepository interface {
	CreateBatch(ctx context.Context, proposals []domain.AnalysisProposal) (int, error)
	GetByID(ctx context.Context, id string) (*domain.AnalysisProposal, error)
	List(ctx context.Context, f repository.ProposalFilter) ([]domain.AnalysisProposal, int64, error)
	CountByStatus(ctx context.Context, workID string) (int64, int64, int64, error)
	Approve(
		ctx context.Context, id, note string, payloadOverride map[string]any,
		apply func(tx *gorm.DB, p domain.AnalysisProposal) (string, error),
	) (*domain.AnalysisProposal, error)
	Reject(ctx context.Context, id, note string) (*domain.AnalysisProposal, error)
}

// PromptRunner 是"跑一次提示词拿回文本"的能力（由 ModelInvoker 实现，测试可替身）。
type PromptRunner interface {
	RunPrompt(ctx context.Context, promptName string, data any) (string, error)
}

// AnalysisService 负责分阶段分析、提案落库与审核应用。
//
// 红线（SPEC.md §22.2）：本服务**不会**在分析阶段直接写原著正式表，
// 只有 ApproveProposal 在作者确认后才写，且与状态更新在同一事务内。
type AnalysisService struct {
	proposals AnalysisProposalRepository
	works     WorkLookup
	chapters  ChapterReader
	tasks     *TaskService
}

// ChapterReader 取章节正文（含内容）的能力。
type ChapterReader interface {
	ListChapterContents(ctx context.Context, workID string, limit int) ([]domain.OriginalChapter, error)
}

// ErrNoChapters 表示原著还没有章节，无法分析。
var ErrNoChapters = errors.New("该原著还没有章节，请先导入原文")

// NewAnalysisService 构建服务。
func NewAnalysisService(
	proposals AnalysisProposalRepository,
	works WorkLookup,
	chapters ChapterReader,
	tasks *TaskService,
) *AnalysisService {
	return &AnalysisService{proposals: proposals, works: works, chapters: chapters, tasks: tasks}
}

// EnqueueStage 把某个分析阶段排成异步任务。
func (s *AnalysisService) EnqueueStage(ctx context.Context, workID string, stage domain.AnalysisStage) (*domain.Task, error) {
	if !stage.Valid() {
		return nil, fmt.Errorf("%w: %s", domain.ErrProposalStageInvalid, stage)
	}
	work, err := s.works.GetWorkByID(ctx, workID)
	if err != nil {
		return nil, err
	}
	if work.ChapterCount == 0 {
		return nil, ErrNoChapters
	}
	projectID := work.ProjectID
	input := map[string]any{"work_id": work.ID, "stage": string(stage)}
	if stage == domain.StageChapterSummary {
		input["chapter_limit"] = 30
	}
	return s.tasks.Enqueue(ctx, EnqueueInput{
		Type:      stage.TaskType(),
		ProjectID: &projectID,
		WorkID:    &work.ID,
		Input:     input,
	})
}

// StageResult 是一个分析阶段的产出摘要。
type StageResult struct {
	Stage        domain.AnalysisStage
	CreatedCount int
	ModelReply   string
}

// RunStage 执行一个分析阶段：组上下文 → 调模型 → 解析 JSON → 落提案（不写正式表）。
func (s *AnalysisService) RunStage(
	ctx context.Context,
	workID string,
	taskID *string,
	stage domain.AnalysisStage,
	runner PromptRunner,
	progress func(stage string, percent int),
) (*StageResult, error) {
	if !stage.Valid() {
		return nil, fmt.Errorf("%w: %s", domain.ErrProposalStageInvalid, stage)
	}
	work, err := s.works.GetWorkByID(ctx, workID)
	if err != nil {
		return nil, err
	}
	if progress != nil {
		progress("读取章节", 10)
	}

	switch stage {
	case domain.StageChapterSummary:
		return s.runChapterSummary(ctx, work, taskID, runner, progress)
	case domain.StageCharacterExtract, domain.StageWorldExtract, domain.StagePlotExtract:
		return s.runExtract(ctx, work, taskID, stage, runner, progress)
	default:
		return nil, fmt.Errorf("尚未实现的阶段: %s", stage)
	}
}

const (
	// 单次送入模型的原文上限（按字符数），避免把整本书塞进上下文
	maxAnalysisChars    = 12000
	maxAnalysisChapters = 20
)

func (s *AnalysisService) runChapterSummary(
	ctx context.Context, work *domain.OriginalWork, taskID *string,
	runner PromptRunner, progress func(string, int),
) (*StageResult, error) {
	limit := 30
	chapters, err := s.chapters.ListChapterContents(ctx, work.ID, limit)
	if err != nil {
		return nil, err
	}
	if len(chapters) == 0 {
		return nil, domain.ErrImportEmpty
	}

	var proposals []domain.AnalysisProposal
	var lastReply string
	for i, chapter := range chapters {
		if progress != nil {
			progress(fmt.Sprintf("概括第 %d/%d 章", i+1, len(chapters)), 10+int(float64(i)/float64(len(chapters))*80))
		}
		reply, err := runner.RunPrompt(ctx, "chapter_summary", map[string]any{
			"ChapterTitle": chapter.Title,
			"ChapterText":  trimChars(chapter.Content, maxAnalysisChars),
		})
		if err != nil {
			return nil, fmt.Errorf("第 %d 章概括失败: %w", chapter.ChapterNo, err)
		}
		lastReply = reply
		obj, err := ExtractJSONObject(reply)
		if err != nil {
			// 单章解析失败不拖垮整批：跳过并在返回里体现（created 会少一条）
			continue
		}
		payload := map[string]any{
			"chapter_no":    chapter.ChapterNo,
			"chapter_title": chapter.Title,
			"summary":       strVal(obj, "summary"),
			"key_points":    obj["key_points"],
			"characters":    obj["characters_appearing"],
			"locations":     obj["locations"],
		}
		proposals = append(proposals, domain.AnalysisProposal{
			WorkID: work.ID, TaskID: taskID, Stage: domain.StageChapterSummary,
			EntityType: domain.EntityChapterSummary,
			Title:      fmt.Sprintf("第 %d 章 %s", chapter.ChapterNo, chapter.Title),
			Payload:    payload,
		})
	}

	created, err := s.proposals.CreateBatch(ctx, proposals)
	if err != nil {
		return nil, err
	}
	return &StageResult{Stage: domain.StageChapterSummary, CreatedCount: created, ModelReply: lastReply}, nil
}

func (s *AnalysisService) runExtract(
	ctx context.Context, work *domain.OriginalWork, taskID *string,
	stage domain.AnalysisStage, runner PromptRunner, progress func(string, int),
) (*StageResult, error) {
	chapters, err := s.chapters.ListChapterContents(ctx, work.ID, maxAnalysisChapters)
	if err != nil {
		return nil, err
	}
	if len(chapters) == 0 {
		return nil, domain.ErrImportEmpty
	}
	if progress != nil {
		progress("组装上下文", 20)
	}

	// 按字符预算截断，保证 prompt 不会超长
	budget := maxAnalysisChars
	items := make([]map[string]any, 0, len(chapters))
	for _, c := range chapters {
		if budget <= 0 {
			break
		}
		text := c.Content
		if len([]rune(text)) > budget {
			text = string([]rune(text)[:budget])
		}
		budget -= len([]rune(text))
		items = append(items, map[string]any{"Title": c.Title, "Text": text})
	}

	promptName := string(stage)
	if progress != nil {
		progress("调用模型", 40)
	}
	reply, err := runner.RunPrompt(ctx, promptName, map[string]any{
		"WorkTitle":    work.Title,
		"ChapterCount": len(chapters),
		"Chapters":     items,
	})
	if err != nil {
		return nil, err
	}
	if progress != nil {
		progress("解析模型输出", 70)
	}
	obj, err := ExtractJSONObject(reply)
	if err != nil {
		return nil, err
	}

	proposals := buildProposals(stage, work.ID, taskID, obj)
	created, err := s.proposals.CreateBatch(ctx, proposals)
	if err != nil {
		return nil, err
	}
	return &StageResult{Stage: stage, CreatedCount: created, ModelReply: reply}, nil
}

// buildProposals 把模型输出里各实体数组转成提案。
func buildProposals(stage domain.AnalysisStage, workID string, taskID *string, obj map[string]any) []domain.AnalysisProposal {
	var out []domain.AnalysisProposal
	appendItems := func(key string, entity domain.ProposalEntity, titleKey string) {
		for _, raw := range asSlice(obj[key]) {
			item, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			title := strVal(item, titleKey)
			if strings.TrimSpace(title) == "" {
				continue
			}
			confidence := 0
			if v, ok := item["confidence"].(float64); ok {
				confidence = int(v)
			}
			out = append(out, domain.AnalysisProposal{
				WorkID: workID, TaskID: taskID, Stage: stage, EntityType: entity,
				Title: title, Payload: item, Evidence: strVal(item, "evidence"), Confidence: confidence,
			})
		}
	}

	appendItems("characters", domain.EntityCharacter, "name")
	if world, ok := obj["world"].(map[string]any); ok && len(world) > 0 {
		out = append(out, domain.AnalysisProposal{
			WorkID: workID, TaskID: taskID, Stage: stage, EntityType: domain.EntityWorld,
			Title: strVal(world, "name"), Payload: world, Evidence: strVal(world, "evidence"),
		})
	}
	appendItems("rules", domain.EntityWorldRule, "name")
	appendItems("locations", domain.EntityLocation, "name")
	appendItems("factions", domain.EntityFaction, "name")
	appendItems("events", domain.EntityEvent, "title")
	appendItems("plot_arcs", domain.EntityPlotArc, "title")
	return out
}

// ---------- 提案查询与审核 ----------

// ListProposals 列出提案。
func (s *AnalysisService) ListProposals(ctx context.Context, f repository.ProposalFilter) ([]domain.AnalysisProposal, int64, error) {
	items, total, err := s.proposals.List(ctx, f)
	if err != nil {
		return nil, 0, err
	}
	if items == nil {
		items = []domain.AnalysisProposal{}
	}
	return items, total, nil
}

// GetProposal 取提案。
func (s *AnalysisService) GetProposal(ctx context.Context, id string) (*domain.AnalysisProposal, error) {
	if strings.TrimSpace(id) == "" {
		return nil, domain.ErrProposalNotFound
	}
	return s.proposals.GetByID(ctx, id)
}

// ProposalSummary 是提案统计。
type ProposalSummary struct {
	Pending  int64
	Approved int64
	Rejected int64
}

// Summary 统计某原著的提案状态。
func (s *AnalysisService) Summary(ctx context.Context, workID string) (*ProposalSummary, error) {
	if _, err := s.works.GetWorkByID(ctx, workID); err != nil {
		return nil, err
	}
	pending, approved, rejected, err := s.proposals.CountByStatus(ctx, workID)
	if err != nil {
		return nil, err
	}
	return &ProposalSummary{Pending: pending, Approved: approved, Rejected: rejected}, nil
}

// ApproveProposal 审核通过：可带作者修改后的 payload，写入正式表。
func (s *AnalysisService) ApproveProposal(
	ctx context.Context,
	id string,
	payloadOverride map[string]any,
	note string,
) (*domain.AnalysisProposal, error) {
	return s.proposals.Approve(ctx, id, note, payloadOverride, s.applyProposal)
}

// RejectProposal 驳回提案。
func (s *AnalysisService) RejectProposal(ctx context.Context, id, note string) (*domain.AnalysisProposal, error) {
	return s.proposals.Reject(ctx, id, note)
}

// applyProposal 把提案内容写入对应的原著正式表（在同事务的 tx 上执行）。
func (s *AnalysisService) applyProposal(tx *gorm.DB, p domain.AnalysisProposal) (string, error) {
	ctx := context.Background()
	switch p.EntityType {
	case domain.EntityChapterSummary:
		chapterNo := intVal(p.Payload, "chapter_no")
		summary := strVal(p.Payload, "summary")
		if chapterNo <= 0 || summary == "" {
			return "", fmt.Errorf("%w：章节摘要缺少 chapter_no 或 summary", domain.ErrProposalPayloadInvalid)
		}
		if err := repository.NewOriginalRepo(tx).UpdateChapterSummary(ctx, p.WorkID, chapterNo, summary); err != nil {
			return "", err
		}
		return p.WorkID, nil

	case domain.EntityCharacter:
		character := &domain.OriginalCharacter{
			OriginalWorkID:   p.WorkID,
			Name:             strVal(p.Payload, "name"),
			Aliases:          strSlice(p.Payload["aliases"]),
			Role:             strVal(p.Payload, "role"),
			Gender:           strVal(p.Payload, "gender"),
			Age:              strVal(p.Payload, "age"),
			Appearance:       strVal(p.Payload, "appearance"),
			Personality:      strVal(p.Payload, "personality"),
			Motivation:       strVal(p.Payload, "motivation"),
			Values:           strVal(p.Payload, "values"),
			Fears:            strVal(p.Payload, "fears"),
			Desires:          strVal(p.Payload, "desires"),
			BehaviorPatterns: strVal(p.Payload, "behavior_patterns"),
			SpeechStyle:      strVal(p.Payload, "speech_style"),
			Abilities:        strVal(p.Payload, "abilities"),
			FirstAppearance:  strVal(p.Payload, "first_appearance"),
			LastAppearance:   strVal(p.Payload, "last_appearance"),
			Importance:       intVal(p.Payload, "importance"),
			Source:           domain.SourceAI,
			DNA:              parseDNA(p.Payload["dna"]),
		}
		character.Normalize()
		if err := character.Validate(); err != nil {
			return "", err
		}
		if err := repository.NewOriginalCharacterRepo(tx).CreateCharacter(ctx, character); err != nil {
			return "", err
		}
		return character.ID, nil

	case domain.EntityWorld:
		world := &domain.OriginalWorld{OriginalWorkID: p.WorkID}
		world.Name = strVal(p.Payload, "name")
		world.Description = strVal(p.Payload, "description")
		world.Normalize()
		if err := repository.NewOriginalWorldRepo(tx).UpsertWorld(ctx, world); err != nil {
			return "", err
		}
		return world.ID, nil

	case domain.EntityWorldRule:
		worldRepo := repository.NewOriginalWorldRepo(tx)
		world, err := ensureWorldTx(ctx, worldRepo, p.WorkID)
		if err != nil {
			return "", err
		}
		rule := &domain.WorldRule{
			WorldID: world.ID, Category: strVal(p.Payload, "category"),
			Name: strVal(p.Payload, "name"), Description: strVal(p.Payload, "description"),
			Importance: intVal(p.Payload, "importance"),
		}
		rule.Normalize()
		if err := rule.Validate(); err != nil {
			return "", err
		}
		if err := worldRepo.CreateRule(ctx, rule); err != nil {
			return "", err
		}
		return rule.ID, nil

	case domain.EntityLocation:
		worldRepo := repository.NewOriginalWorldRepo(tx)
		world, err := ensureWorldTx(ctx, worldRepo, p.WorkID)
		if err != nil {
			return "", err
		}
		location := &domain.Location{
			WorldID: world.ID, Name: strVal(p.Payload, "name"),
			Type: strVal(p.Payload, "type"), Description: strVal(p.Payload, "description"),
		}
		// 上级地点按名字匹配已有地点（匹配不到就当作顶层）
		if parentName := strVal(p.Payload, "parent"); parentName != "" {
			existing, err := worldRepo.ListLocations(ctx, world.ID)
			if err == nil {
				for _, l := range existing {
					if strings.EqualFold(l.Name, parentName) {
						id := l.ID
						location.ParentLocationID = &id
						break
					}
				}
			}
		}
		location.Normalize()
		if err := location.Validate(); err != nil {
			return "", err
		}
		if err := worldRepo.CreateLocation(ctx, location); err != nil {
			return "", err
		}
		return location.ID, nil

	case domain.EntityFaction:
		worldRepo := repository.NewOriginalWorldRepo(tx)
		world, err := ensureWorldTx(ctx, worldRepo, p.WorkID)
		if err != nil {
			return "", err
		}
		faction := &domain.Faction{
			WorldID: world.ID, Name: strVal(p.Payload, "name"), Type: strVal(p.Payload, "type"),
			Description: strVal(p.Payload, "description"), Goals: strVal(p.Payload, "goals"),
			Relationships: strVal(p.Payload, "relationships"),
		}
		faction.Normalize()
		if err := faction.Validate(); err != nil {
			return "", err
		}
		if err := worldRepo.CreateFaction(ctx, faction); err != nil {
			return "", err
		}
		return faction.ID, nil

	case domain.EntityEvent:
		eventRepo := repository.NewOriginalEventRepo(tx)
		characterRepo := repository.NewOriginalCharacterRepo(tx)

		// 参与者按姓名解析成人物 ID（解析不到就忽略，不阻断）
		participants := make([]string, 0)
		if names := strSlice(p.Payload["participants"]); len(names) > 0 {
			characters, _, err := characterRepo.ListCharacters(ctx, p.WorkID, repository.CharacterFilter{PageSize: 200})
			if err == nil {
				for _, name := range names {
					for _, c := range characters {
						if strings.EqualFold(c.Name, name) {
							participants = append(participants, c.ID)
							break
						}
					}
				}
			}
		}

		var chapterNo *int
		if n := intVal(p.Payload, "chapter_no"); n > 0 {
			chapterNo = &n
		}
		event := &domain.OriginalEvent{
			OriginalWorkID: p.WorkID,
			Title:          strVal(p.Payload, "title"),
			Description:    strVal(p.Payload, "description"),
			ChapterNo:      chapterNo,
			TimeOrder:      intVal(p.Payload, "time_order"),
			Participants:   participants,
			LocationText:   strVal(p.Payload, "location"),
			Consequences:   strVal(p.Payload, "consequences"),
			Importance:     intVal(p.Payload, "importance"),
			Source:         domain.SourceAI,
		}
		// 地点名若能匹配到世界观地点则建立关联
		if name := strVal(p.Payload, "location"); name != "" {
			if world, err := repository.NewOriginalWorldRepo(tx).GetWorld(ctx, p.WorkID); err == nil {
				if locations, err := repository.NewOriginalWorldRepo(tx).ListLocations(ctx, world.ID); err == nil {
					for _, l := range locations {
						if strings.EqualFold(l.Name, name) {
							id := l.ID
							event.LocationID = &id
							break
						}
					}
				}
			}
		}
		event.Normalize()
		if err := event.Validate(); err != nil {
			return "", err
		}
		if err := eventRepo.CreateEvent(ctx, event); err != nil {
			return "", err
		}
		return event.ID, nil

	case domain.EntityPlotArc:
		eventRepo := repository.NewOriginalEventRepo(tx)
		arc := &domain.PlotArc{
			OriginalWorkID: p.WorkID,
			Type:           domain.PlotArcType(strVal(p.Payload, "type")),
			Title:          strVal(p.Payload, "title"),
			Summary:        strVal(p.Payload, "summary"),
		}
		// 起止事件按标题匹配
		if events, _, err := eventRepo.ListEvents(ctx, p.WorkID, repository.EventFilter{PageSize: 500}); err == nil {
			resolve := func(key string) *string {
				name := strVal(p.Payload, key)
				if name == "" {
					return nil
				}
				for _, e := range events {
					if strings.EqualFold(e.Title, name) {
						id := e.ID
						return &id
					}
				}
				return nil
			}
			arc.StartEventID = resolve("start_event")
			arc.EndEventID = resolve("end_event")
		}
		arc.Normalize()
		if err := arc.Validate(); err != nil {
			return "", err
		}
		if err := eventRepo.CreatePlotArc(ctx, arc); err != nil {
			return "", err
		}
		return arc.ID, nil

	default:
		return "", fmt.Errorf("%w: 暂不支持审核写入的实体类型 %s", domain.ErrProposalEntityInvalid, p.EntityType)
	}
}

func ensureWorldTx(ctx context.Context, repo *repository.OriginalWorldRepo, workID string) (*domain.OriginalWorld, error) {
	world, err := repo.GetWorld(ctx, workID)
	if err == nil {
		return world, nil
	}
	if !errors.Is(err, domain.ErrWorldNotFound) {
		return nil, err
	}
	created := &domain.OriginalWorld{OriginalWorkID: workID}
	if err := repo.UpsertWorld(ctx, created); err != nil {
		return nil, err
	}
	return created, nil
}

// ---------- 小工具 ----------

func asSlice(v any) []any {
	if s, ok := v.([]any); ok {
		return s
	}
	return nil
}

func strVal(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	if s, ok := m[key].(string); ok {
		return strings.TrimSpace(s)
	}
	return ""
}

func intVal(m map[string]any, key string) int {
	if m == nil {
		return 0
	}
	switch v := m[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case string:
		var n int
		_, err := fmt.Sscanf(v, "%d", &n)
		if err == nil {
			return n
		}
	}
	return 0
}

func strSlice(v any) []string {
	items := asSlice(v)
	if items == nil {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, raw := range items {
		if s, ok := raw.(string); ok && strings.TrimSpace(s) != "" {
			out = append(out, strings.TrimSpace(s))
		}
	}
	return out
}

// parseDNA 把模型返回的 dna 对象转成 CharacterDNA（字段缺失自动补零值）。
func parseDNA(v any) domain.CharacterDNA {
	var dna domain.CharacterDNA
	if v == nil {
		return dna
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return dna
	}
	if err := json.Unmarshal(raw, &dna); err != nil {
		return domain.CharacterDNA{}
	}
	return dna
}

func trimChars(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}
