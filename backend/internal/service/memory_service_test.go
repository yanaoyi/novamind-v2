package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/yanaoyi/novamindv2/backend/internal/domain"
	"github.com/yanaoyi/novamindv2/backend/internal/repository"
)

// 这组测试锁的是 Phase 9 §9.3.2：抽取 → 入库 → 写索引 → 自动查一致性。

type fakeMemoryStore struct {
	writes    []repository.FactWrite
	summaries map[string]string
	err       error
}

func (f *fakeMemoryStore) CreateFacts(
	_ context.Context, _, _ string, facts []repository.FactWrite,
) (repository.FactWriteOutcome, error) {
	if f.err != nil {
		return repository.FactWriteOutcome{}, f.err
	}
	f.writes = append(f.writes, facts...)
	out := repository.FactWriteOutcome{}
	for i, fact := range facts {
		out.Created = append(out.Created, domain.MemoryFact{
			ID: fmt.Sprintf("f%d", i+1), Kind: domain.FactKind(fact.Kind),
			Subject: fact.Subject, Fact: fact.Fact, ChapterID: fact.ChapterID,
		})
	}
	return out, nil
}

func (f *fakeMemoryStore) ListFacts(context.Context, string, bool) ([]domain.MemoryFact, error) {
	return nil, nil
}

func (f *fakeMemoryStore) UpsertSummary(_ context.Context, _, chapterID, summary string) error {
	if f.summaries == nil {
		f.summaries = map[string]string{}
	}
	f.summaries[chapterID] = summary
	return nil
}

func (f *fakeMemoryStore) GetSummary(context.Context, string) (*domain.ChapterSummary, error) {
	return nil, domain.ErrChapterSummaryNotFound
}

type fakeMemoryIndexer struct{ texts []string }

func (f *fakeMemoryIndexer) IndexText(
	_ context.Context, _, _, refKind, refID, text string, _ *string,
) error {
	f.texts = append(f.texts, refKind+"/"+refID+"/"+text)
	return nil
}

type fakeChecks struct{ calls []string }

func (f *fakeChecks) EnqueueConsistencyCheck(_ context.Context, workID string, chapterIDs []string) error {
	f.calls = append(f.calls, workID+":"+strings.Join(chapterIDs, ","))
	return nil
}

// scriptedRunner 按顺序返回预设回复，并记下每次拿到的模板变量。
type scriptedRunner struct {
	replies []string
	seen    []map[string]any
	i       int
}

func (r *scriptedRunner) next(data any) (string, error) {
	if m, ok := data.(map[string]any); ok {
		r.seen = append(r.seen, m)
	}
	if r.i >= len(r.replies) {
		return "", errors.New("没有更多预设回复")
	}
	out := r.replies[r.i]
	r.i++
	return out, nil
}

func (r *scriptedRunner) RunPrompt(_ context.Context, _ string, data any) (string, error) {
	return r.next(data)
}

func (r *scriptedRunner) RunTextPrompt(_ context.Context, _ string, data any) (string, error) {
	return r.next(data)
}

func newMemoryServiceForTest(chapter *domain.CreativeChapter) (*MemoryService, *fakeMemoryStore, *fakeMemoryIndexer, *fakeChecks, *stubWritingRepo) {
	store := &fakeMemoryStore{}
	indexer := &fakeMemoryIndexer{}
	checks := &fakeChecks{}
	repo := &stubWritingRepo{chapter: chapter, chapters: []domain.CreativeChapter{*chapter}}
	svc := NewMemoryService(store, repo, &stubContextReader{}, fakeOwnerResolver{id: "owner-1"}, indexer)
	svc.SetConsistencyTrigger(checks)
	return svc, store, indexer, checks, repo
}

const validFactReply = `{"facts":[{"kind":"character_state","subject":"沈砚","fact":"左臂受伤，无法用剑"},
  {"kind":"item","subject":"青铜钥匙","fact":"从祖宅第三块砖下取出，随身携带"}],
  "summary":"沈砚回到老宅，取走青铜钥匙。"}`

func TestExtractFactsPersistsIndexesAndTriggersCheck(t *testing.T) {
	chapter := &domain.CreativeChapter{
		ID: "ch-1", CreativeWorkID: "cw-1", ChapterNo: 3, Title: "雨夜归人",
		Content: "沈砚推开老宅的门，怀里揣着青铜钥匙。", Purpose: "交代钥匙来源",
	}
	svc, store, indexer, checks, _ := newMemoryServiceForTest(chapter)
	runner := &scriptedRunner{replies: []string{validFactReply}}

	result, err := svc.ExtractFacts(context.Background(), "ch-1", runner, nil)
	if err != nil {
		t.Fatalf("抽取失败: %v", err)
	}
	if result.FactsCreated != 2 || result.Indexed != 3 {
		t.Fatalf("应写入 2 条事实并索引 3 个来源（2 事实 + 1 摘要），实际 %d/%d",
			result.FactsCreated, result.Indexed)
	}
	if len(store.writes) != 2 || store.writes[0].Kind != string(domain.FactCharacterState) {
		t.Errorf("事实应带类型写入，实际 %#v", store.writes)
	}
	if store.writes[0].ChapterID == nil || *store.writes[0].ChapterID != "ch-1" {
		t.Error("事实应记录来源章节（回答“这条记忆是哪一章确立的”）")
	}
	if !strings.Contains(store.summaries["ch-1"], "青铜钥匙") {
		t.Errorf("章节摘要应入库，实际 %v", store.summaries)
	}
	joined := strings.Join(indexer.texts, " | ")
	if !strings.Contains(joined, "memory_fact/f1/") || !strings.Contains(joined, "chapter_summary/ch-1/") {
		t.Errorf("事实与摘要都要写进检索索引，实际 %s", joined)
	}
	if len(checks.calls) != 1 || checks.calls[0] != "cw-1:ch-1" {
		t.Errorf("抽取完应自动触发一次本章一致性检查，实际 %v", checks.calls)
	}
	// 模板变量：正文、章号、目标都要在（模板是 missingkey=error，缺一个就渲染失败）
	vars := runner.seen[0]
	for _, key := range []string{"ChapterNo", "ChapterTitle", "ChapterGoal", "CharacterContext", "WorldContext", "ChapterText"} {
		if _, ok := vars[key]; !ok {
			t.Errorf("fact_extract 模板变量缺 %s", key)
		}
	}
}

func TestExtractFactsRepairsSchemaViolationOnce(t *testing.T) {
	chapter := &domain.CreativeChapter{ID: "ch-2", CreativeWorkID: "cw-1", ChapterNo: 4, Title: "试探", Content: "正文"}
	svc, store, _, checks, _ := newMemoryServiceForTest(chapter)
	badKind := `{"facts":[{"kind":"mood","subject":"沈砚","fact":"心情不错"}],"summary":"x"}`
	runner := &scriptedRunner{replies: []string{badKind, validFactReply}}

	result, err := svc.ExtractFacts(context.Background(), "ch-2", runner, nil)
	if err != nil {
		t.Fatalf("第二次输出合法时应成功: %v", err)
	}
	if result.FactsCreated != 2 {
		t.Errorf("应以第二次输出为准，实际新增 %d 条", result.FactsCreated)
	}
	if len(runner.seen) != 2 {
		t.Fatalf("应重试一次，实际调用 %d 次", len(runner.seen))
	}
	// 校验问题必须喂回模型（§9.3.2：失败则把校验错误喂回 LLM 修一次）
	hint, _ := runner.seen[1]["Instruction"].(string)
	if !strings.Contains(hint, "没有通过结构校验") || !strings.Contains(hint, "character_state") {
		t.Errorf("重试时应把校验器的具体问题喂回模型，实际 %q", hint)
	}
	if len(checks.calls) != 1 {
		t.Errorf("修复后仍应触发一致性检查，实际 %v", checks.calls)
	}
	_ = store
}

func TestExtractFactsGivesUpWithoutPersisting(t *testing.T) {
	chapter := &domain.CreativeChapter{ID: "ch-3", CreativeWorkID: "cw-1", ChapterNo: 5, Title: "夜审", Content: "正文"}
	svc, store, indexer, checks, _ := newMemoryServiceForTest(chapter)
	bad := `{"facts":"无","summary":""}`
	runner := &scriptedRunner{replies: []string{bad, bad}}

	if _, err := svc.ExtractFacts(context.Background(), "ch-3", runner, nil); err == nil {
		t.Fatal("两次都不合格时应报错（任务记为 FAILED，不阻塞写作）")
	}
	if len(store.writes) != 0 || len(indexer.texts) != 0 || len(checks.calls) != 0 {
		t.Errorf("输出不合法时不该写入任何记忆或索引，实际 %d/%d/%d",
			len(store.writes), len(indexer.texts), len(checks.calls))
	}
}

func TestExtractFactsRejectsEmptyChapterWithoutCallingModel(t *testing.T) {
	chapter := &domain.CreativeChapter{ID: "ch-4", CreativeWorkID: "cw-1", ChapterNo: 6, Title: "空章"}
	svc, _, _, _, _ := newMemoryServiceForTest(chapter)
	runner := &scriptedRunner{replies: []string{validFactReply}}

	if _, err := svc.ExtractFacts(context.Background(), "ch-4", runner, nil); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("空正文应返回 ErrBadRequest，实际 %v", err)
	}
	if runner.i != 0 {
		t.Error("空正文不该白调一次模型")
	}
}

type fakeFactTrigger struct{ calls []string }

func (f *fakeFactTrigger) EnqueueFactExtract(_ context.Context, workID, chapterID string) error {
	f.calls = append(f.calls, workID+"/"+chapterID)
	return nil
}

func TestChapterSaveTriggersFactExtractionToo(t *testing.T) {
	chapter := &domain.CreativeChapter{ID: "ch-5", CreativeWorkID: "cw-1", Title: "开局", ChapterNo: 1, Content: "旧"}
	repo := &stubWritingRepo{chapter: chapter}
	svc := NewWritingService(repo, &stubCreativeReader{}, &stubContextReader{})
	indexTrigger := &fakeIndexTrigger{}
	factTrigger := &fakeFactTrigger{}
	svc.SetIndexTrigger(indexTrigger)
	svc.SetFactExtractTrigger(factTrigger)

	content := "新正文"
	if _, _, err := svc.UpdateChapter(context.Background(), "ch-5", UpdateChapterInput{Content: &content}); err != nil {
		t.Fatalf("改正文失败: %v", err)
	}
	if len(indexTrigger.calls) != 1 || len(factTrigger.calls) != 1 {
		t.Fatalf("正文变化应同时触发索引与记忆抽取，实际 %v / %v", indexTrigger.calls, factTrigger.calls)
	}
	if factTrigger.calls[0] != "cw-1/ch-5" {
		t.Errorf("记忆抽取应带作品与章节，实际 %v", factTrigger.calls)
	}
}
