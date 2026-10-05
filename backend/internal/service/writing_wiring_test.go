package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/yanaoyi/novamindv2/backend/internal/ai"
	ctxengine "github.com/yanaoyi/novamindv2/backend/internal/context"
	"github.com/yanaoyi/novamindv2/backend/internal/domain"
	"github.com/yanaoyi/novamindv2/backend/internal/retrieval"
)

// 这组测试锁的是 Phase 9 §9.2 的"接线"本身：检索 → 8 段组装 → 提示词 → 快照。
// 全部用桩件，不碰数据库、不调模型。

// stubWritingRepo 只实现本组测试会用到的方法；其余方法由嵌入的接口兜底
// （真去调没实现的方法会 panic，这正是我们想要的"别悄悄走错路"）。
type stubWritingRepo struct {
	WritingRepository
	chapter  *domain.CreativeChapter
	chapters []domain.CreativeChapter
	issues   []domain.ConsistencyIssue
}

func (r *stubWritingRepo) GetChapter(_ context.Context, id string) (*domain.CreativeChapter, error) {
	if r.chapter == nil || r.chapter.ID != id {
		return nil, errors.New("章节不存在")
	}
	return r.chapter, nil
}

func (r *stubWritingRepo) ListChaptersBefore(_ context.Context, _ string, _ int, _ int) ([]domain.CreativeChapter, error) {
	return nil, nil
}

func (r *stubWritingRepo) ListChapters(_ context.Context, _ string, _ bool) ([]domain.CreativeChapter, error) {
	return r.chapters, nil
}

func (r *stubWritingRepo) CreateIssues(_ context.Context, issues []domain.ConsistencyIssue) (int, error) {
	r.issues = append(r.issues, issues...)
	return len(issues), nil
}

type stubCreativeReader struct {
	CreativeWorkReader
	work *domain.CreativeWork
}

func (r *stubCreativeReader) GetWorkByID(_ context.Context, _ string) (*domain.CreativeWork, error) {
	return r.work, nil
}

type stubContextReader struct {
	ChapterContextReader
	timeline []domain.CreativeTimelineEvent
}

func (r *stubContextReader) ListCharacters(_ context.Context, _ string) ([]domain.CreativeCharacter, error) {
	return []domain.CreativeCharacter{{Name: "沈砚", Description: "克制"}}, nil
}

func (r *stubContextReader) GetTimeline(_ context.Context, _ string) ([]domain.CreativeTimelineEvent, error) {
	return r.timeline, nil
}

func (r *stubContextReader) GetWorldDetail(_ context.Context, workID string) (*WorldDetail, error) {
	return &WorldDetail{
		CreativeWorkID: workID,
		World:          &domain.CreativeWorld{Name: "江城"},
		Rules: []domain.CreativeWorldRule{
			{Status: domain.RuleInherited, Category: "社会", Name: "夜禁", Description: "子时后不得出城"},
		},
	}, nil
}

func (r *stubContextReader) ListMappings(_ context.Context, _ string) ([]domain.OriginalCreativeMapping, error) {
	return []domain.OriginalCreativeMapping{
		{OriginalType: "character", OriginalID: "A", CreativeType: "character", CreativeID: "B", MappingType: domain.MappingInherited},
	}, nil
}

// capturingRunner 记下服务传进来的模板变量（快照与提示词必须同源）。
type capturingRunner struct {
	promptName string
	vars       map[string]any
	text       string
	model      string
}

func (r *capturingRunner) RunPrompt(_ context.Context, name string, data any) (string, error) {
	r.promptName = name
	if m, ok := data.(map[string]any); ok {
		r.vars = m
	}
	return `{"issues":[]}`, nil
}

func (r *capturingRunner) RunTextPrompt(_ context.Context, name string, data any) (string, error) {
	r.promptName = name
	if m, ok := data.(map[string]any); ok {
		r.vars = m
	}
	return r.text, nil
}

// DescribePrompt 让快照能记到真实模型与模板版本（未实现该接口的 runner 会记 unknown）。
//
// 版本号直接取自真实引擎：这样测试断言的是"服务实际会写进快照的版本"，
// 而不是测试自己编的一个数字（加了新模板版本时，断言必须跟着改，改不动就说明有漏项）。
func (r *capturingRunner) DescribePrompt(_ context.Context, name string) (string, string) {
	version := "v1"
	if engine, err := ai.NewEngine(); err == nil {
		if p, err := engine.Get(name, ""); err == nil {
			version = p.Version
		}
	}
	if r.model == "" {
		return "deepseek-chat", version
	}
	return r.model, version
}

type recordedSnapshot struct {
	chapterID *string
	kind      domain.SnapshotKind
	payload   map[string]any
}

type recordingSnapshots struct{ rows []recordedSnapshot }

func (r *recordingSnapshots) Record(
	_ context.Context, chapterID *string, kind domain.SnapshotKind, payload map[string]any,
) (*domain.ContextSnapshot, error) {
	r.rows = append(r.rows, recordedSnapshot{chapterID: chapterID, kind: kind, payload: payload})
	return &domain.ContextSnapshot{}, nil
}

type fakeRetriever struct {
	calls  []string
	byKind map[string][]retrieval.ScoredChunk
	err    error
}

func (f *fakeRetriever) Search(
	_ context.Context, workKind, workID, _ string, _ int,
) ([]retrieval.ScoredChunk, error) {
	f.calls = append(f.calls, workKind+"/"+workID)
	if f.err != nil {
		return nil, f.err
	}
	return f.byKind[workKind], nil
}

func scoredChunk(id, refKind, content string, score float64) retrieval.ScoredChunk {
	refID := "ref-" + id
	return retrieval.ScoredChunk{
		IndexedChunk: retrieval.IndexedChunk{
			ID: id, RefKind: refKind, RefID: &refID, Seq: 1, Content: content,
		},
		Score:     score,
		MatchFrom: []string{"bm25"},
	}
}

func newWiringService(chapter *domain.CreativeChapter) (*WritingService, *stubWritingRepo, *stubCreativeReader) {
	repo := &stubWritingRepo{chapter: chapter, chapters: []domain.CreativeChapter{*chapter}}
	creative := &stubCreativeReader{work: &domain.CreativeWork{
		ID: chapter.CreativeWorkID, OriginalWorkID: "orig-1", Title: "同人·江城",
	}}
	svc := NewWritingService(repo, creative, &stubContextReader{
		timeline: []domain.CreativeTimelineEvent{
			{Sequence: 1, Status: domain.TimelineNew, TimeLabel: "灯会当夜", Title: "库房起火"},
		},
	})
	return svc, repo, creative
}

func TestGenerateChapterDraftWiresRetrievalIntoPromptAndSnapshot(t *testing.T) {
	chapter := &domain.CreativeChapter{
		ID: "ch-9", CreativeWorkID: "cw-1", Title: "夜审账册", ChapterNo: 9,
		Purpose: "让沈砚发现账册被改", Summary: "他对质账房先生",
	}
	svc, _, _ := newWiringService(chapter)
	rt := &fakeRetriever{byKind: map[string][]retrieval.ScoredChunk{
		domain.WorkKindOriginal: {scoredChunk("c1", domain.ChunkRefChapter, "第一章埋下青铜钥匙。", 0.91)},
		domain.WorkKindCreative: {scoredChunk("c2", domain.ChunkRefMemoryFact, "沈砚左臂受伤。", 0.66)},
	}}
	svc.SetRetriever(rt)
	snaps := &recordingSnapshots{}
	svc.SetSnapshotRecorder(snaps)
	runner := &capturingRunner{text: "正文……"}

	if _, err := svc.GenerateChapterDraft(context.Background(), "ch-9", runner, 1500, "别写成大纲体", nil); err != nil {
		t.Fatalf("生成草稿失败: %v", err)
	}

	// 1) 两路检索都按作品各查一次
	joined := strings.Join(rt.calls, ",")
	if !strings.Contains(joined, "original/orig-1") || !strings.Contains(joined, "creative/cw-1") {
		t.Errorf("检索应按原著与二创各查一次，实际 %v", rt.calls)
	}
	// 2) 提示词里真的带上了检索内容与时间线
	if runner.promptName != "chapter_generate" {
		t.Fatalf("应走 chapter_generate 模板，实际 %s", runner.promptName)
	}
	for name, want := range map[string]string{
		"RetrievedOriginal": "青铜钥匙",
		"RetrievedCreative": "左臂受伤",
		"TimelineContext":   "库房起火",
		"Instruction":       "别写成大纲体",
	} {
		if got, _ := runner.vars[name].(string); !strings.Contains(got, want) {
			t.Errorf("模板变量 %s 应含 %q，实际 %q", name, want, got)
		}
	}
	// 3) 快照记的是同一份内容 + 真实模型/版本 + 可追溯的检索来源
	if len(snaps.rows) != 1 {
		t.Fatalf("应落 1 条快照，实际 %d", len(snaps.rows))
	}
	row := snaps.rows[0]
	if row.kind != domain.SnapshotGenerate {
		t.Errorf("快照 kind 应为 generate，实际 %s", row.kind)
	}
	if row.payload["model"] != "deepseek-chat" || row.payload["prompt_version"] != "chapter_generate.v3" {
		t.Errorf("快照应记真实模型与模板版本，实际 %v / %v",
			row.payload["model"], row.payload["prompt_version"])
	}
	if row.payload["retrieved_creative"] != runner.vars["RetrievedCreative"] {
		t.Error("快照里的检索段必须与提示词里的完全一致")
	}
	sources, ok := row.payload["retrieved_sources"].([]map[string]any)
	if !ok || len(sources) != 2 {
		t.Fatalf("快照应含 2 条检索来源，实际 %#v", row.payload["retrieved_sources"])
	}
	if sources[0]["work_kind"] != domain.WorkKindOriginal || sources[1]["work_kind"] != domain.WorkKindCreative {
		t.Errorf("来源应标注作品归属，实际 %#v", sources)
	}
	budget, ok := row.payload["token_budget"].(map[string]any)
	if !ok || budget["limit"] != ctxengine.TotalBudget {
		t.Errorf("快照应记 token 预算，实际 %#v", row.payload["token_budget"])
	}
	if _, ok := budget["by_section"].(map[string]int); !ok {
		t.Errorf("快照应记每段用量，实际 %#v", budget["by_section"])
	}
}

func TestRetrievalFailureDoesNotBreakWriting(t *testing.T) {
	chapter := &domain.CreativeChapter{ID: "ch-1", CreativeWorkID: "cw-1", Title: "开局", ChapterNo: 1}
	svc, _, _ := newWiringService(chapter)
	svc.SetRetriever(&fakeRetriever{err: errors.New("上游 500")})
	snaps := &recordingSnapshots{}
	svc.SetSnapshotRecorder(snaps)
	runner := &capturingRunner{text: "正文……"}

	if _, err := svc.GenerateChapterDraft(context.Background(), "ch-1", runner, 800, "", nil); err != nil {
		t.Fatalf("检索失败不该打断写作，实际: %v", err)
	}
	if got, _ := runner.vars["RetrievedOriginal"].(string); got != "" {
		t.Errorf("检索失败时检索段应为空，实际 %q", got)
	}
	if sources, _ := snaps.rows[0].payload["retrieved_sources"].([]map[string]any); len(sources) != 0 {
		t.Errorf("检索失败时不该编造来源，实际 %#v", sources)
	}
}

func TestRewriteTextUsesAssembledContext(t *testing.T) {
	chapter := &domain.CreativeChapter{ID: "ch-3", CreativeWorkID: "cw-1", Title: "对峙", ChapterNo: 3}
	svc, _, _ := newWiringService(chapter)
	svc.SetRetriever(&fakeRetriever{byKind: map[string][]retrieval.ScoredChunk{
		domain.WorkKindCreative: {scoredChunk("c2", domain.ChunkRefChapter, "上一章他折断了自己的剑。", 0.8)},
	}})
	snaps := &recordingSnapshots{}
	svc.SetSnapshotRecorder(snaps)
	runner := &capturingRunner{text: "改写后……"}

	_, err := svc.RewriteText(context.Background(), runner, RewriteInput{
		ChapterID: "ch-3", Text: "他握住了剑。", Action: RewriteActionExpand, Instruction: "再狠一点",
	})
	if err != nil {
		t.Fatalf("改写失败: %v", err)
	}
	if runner.promptName != "rewrite" {
		t.Fatalf("应走 rewrite 模板，实际 %s", runner.promptName)
	}
	if got, _ := runner.vars["RetrievedCreative"].(string); !strings.Contains(got, "折断了自己的剑") {
		t.Errorf("改写也应带上检索到的前作片段，实际 %q", got)
	}
	if len(snaps.rows) != 1 || snaps.rows[0].kind != domain.SnapshotExpand {
		t.Fatalf("扩写应落 kind=expand 的快照，实际 %#v", snaps.rows)
	}
	if snaps.rows[0].payload["prompt_version"] != "rewrite.v3" {
		t.Errorf("快照应记 rewrite.v3，实际 %v", snaps.rows[0].payload["prompt_version"])
	}
}

func TestCheckConsistencySnapshotsPerChapter(t *testing.T) {
	chapter := &domain.CreativeChapter{
		ID: "ch-2", CreativeWorkID: "cw-1", Title: "第二章", ChapterNo: 2,
		Content: "他左臂受伤，却仍用双手握剑。",
	}
	svc, _, _ := newWiringService(chapter)
	svc.SetRetriever(&fakeRetriever{byKind: map[string][]retrieval.ScoredChunk{
		domain.WorkKindCreative: {scoredChunk("c9", domain.ChunkRefMemoryFact, "沈砚左臂受伤，无法用剑。", 0.88)},
	}})
	snaps := &recordingSnapshots{}
	svc.SetSnapshotRecorder(snaps)
	runner := &capturingRunner{}

	result, err := svc.CheckConsistency(context.Background(), "cw-1", nil, runner, nil)
	if err != nil {
		t.Fatalf("一致性检查失败: %v", err)
	}
	if result.Checked != 1 {
		t.Fatalf("应检查 1 章，实际 %d", result.Checked)
	}
	if got, _ := runner.vars["RetrievedCreative"].(string); !strings.Contains(got, "无法用剑") {
		t.Errorf("一致性检查应看到被检索到的既有事实，实际 %q", got)
	}
	if len(snaps.rows) != 1 {
		t.Fatalf("每章应落 1 条快照，实际 %d", len(snaps.rows))
	}
	row := snaps.rows[0]
	if row.kind != domain.SnapshotConsistency {
		t.Errorf("快照 kind 应为 consistency，实际 %s", row.kind)
	}
	if row.chapterID == nil || *row.chapterID != "ch-2" {
		t.Errorf("快照应锚到被检查的章节，实际 %v", row.chapterID)
	}
	if row.payload["prompt_version"] != "consistency_check.v3" {
		t.Errorf("快照应记 consistency_check.v3，实际 %v", row.payload["prompt_version"])
	}
}

func TestAnalyzeTextSnapshotsAssembledContext(t *testing.T) {
	chapter := &domain.CreativeChapter{
		ID: "ch-4", CreativeWorkID: "cw-1", Title: "夜谈", ChapterNo: 4,
		Content: "他左臂的伤口又裂开了。",
	}
	svc, _, _ := newWiringService(chapter)
	svc.SetRetriever(&fakeRetriever{byKind: map[string][]retrieval.ScoredChunk{
		domain.WorkKindCreative: {scoredChunk("c7", domain.ChunkRefChapter, "第二章他左臂中箭。", 0.8)},
	}})
	snaps := &recordingSnapshots{}
	svc.SetSnapshotRecorder(snaps)
	runner := &capturingRunner{}

	if _, err := svc.AnalyzeText(context.Background(), runner, AnalyzeTextInput{
		ChapterID: "ch-4", Focus: "看伤情是否连贯",
	}); err != nil {
		t.Fatalf("就地分析失败: %v", err)
	}
	if runner.promptName != "text_analyze" {
		t.Fatalf("应走 text_analyze 模板，实际 %s", runner.promptName)
	}
	if got, _ := runner.vars["RetrievedCreative"].(string); !strings.Contains(got, "左臂中箭") {
		t.Errorf("就地分析应带上检索到的既有内容，实际 %q", got)
	}
	if len(snaps.rows) != 1 || snaps.rows[0].kind != domain.SnapshotAnalyze {
		t.Fatalf("应落 kind=analyze 的快照，实际 %#v", snaps.rows)
	}
	row := snaps.rows[0]
	if row.payload["prompt_version"] != "text_analyze.v2" || row.payload["model"] != "deepseek-chat" {
		t.Errorf("快照应记真实模型与模板版本，实际 %v / %v",
			row.payload["model"], row.payload["prompt_version"])
	}
	var found bool
	if sources, ok := row.payload["retrieved_sources"].([]map[string]any); ok {
		for _, s := range sources {
			if s["chunk_id"] == "c7" {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("快照应能追到检索来源 c7，实际 %#v", row.payload["retrieved_sources"])
	}
}
