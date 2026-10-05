package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/yanaoyi/novamindv2/backend/internal/domain"
	"github.com/yanaoyi/novamindv2/backend/internal/storage"
)

// 这组测试锁的是 Phase 9 §9.1.4 的增量索引与触发点。

type fakeOwnerResolver struct{ id string }

func (f fakeOwnerResolver) DefaultUserID(context.Context) (string, error) { return f.id, nil }

type replaceCall struct {
	workKind  string
	workID    string
	refKind   string
	refID     *string
	chapterID *string
	chunks    []domain.RetrievalChunk
}

type pruneCall struct {
	refKind string
	keep    []string
}

type fakeChunkStore struct {
	calls  []replaceCall
	prunes []pruneCall
}

func (f *fakeChunkStore) ReplaceChunks(
	_ context.Context, _, workKind, workID, refKind string,
	refID, chapterID *string, chunks []domain.RetrievalChunk,
) error {
	f.calls = append(f.calls, replaceCall{
		workKind: workKind, workID: workID, refKind: refKind,
		refID: refID, chapterID: chapterID, chunks: chunks,
	})
	return nil
}

func (f *fakeChunkStore) PruneChunks(_ context.Context, _, _, _, refKind string, keep []string) error {
	f.prunes = append(f.prunes, pruneCall{refKind: refKind, keep: keep})
	return nil
}

type fakeOriginalReader struct {
	all      []domain.OriginalChapter
	listCall int
}

func (f *fakeOriginalReader) ListAllChapterContents(context.Context, string) ([]domain.OriginalChapter, error) {
	f.listCall++
	return f.all, nil
}

type fakeCreativeChapterReader struct {
	all []domain.CreativeChapter
	one *domain.CreativeChapter
	err error
}

func (f *fakeCreativeChapterReader) ListChapters(context.Context, string, bool) ([]domain.CreativeChapter, error) {
	return f.all, nil
}

func (f *fakeCreativeChapterReader) GetChapter(context.Context, string) (*domain.CreativeChapter, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.one, nil
}

func TestIndexRefRebuildsOnlyThatChapter(t *testing.T) {
	store := &fakeChunkStore{}
	reader := &fakeCreativeChapterReader{one: &domain.CreativeChapter{
		ID: "ch-1", CreativeWorkID: "cw-1",
		Content: "沈砚推开老宅的门。" + strings.Repeat("雨下了一整夜。", 200),
	}}
	svc := NewIndexService(store, fakeOwnerResolver{id: "owner-1"},
		&fakeOriginalReader{}, reader, nil, nil)

	result, err := svc.IndexRef(context.Background(), domain.WorkKindCreative, "cw-1", domain.ChunkRefChapter, "ch-1")
	if err != nil {
		t.Fatalf("增量索引失败: %v", err)
	}
	if result.Items != 1 {
		t.Errorf("只该处理 1 个来源，实际 %d", result.Items)
	}
	if len(store.calls) != 1 {
		t.Fatalf("只该写 1 次分块（只重建这一章），实际 %d", len(store.calls))
	}
	call := store.calls[0]
	if call.workKind != domain.WorkKindCreative || call.workID != "cw-1" || call.refKind != domain.ChunkRefChapter {
		t.Errorf("分块归属不对: %#v", call)
	}
	if call.refID == nil || *call.refID != "ch-1" || call.chapterID == nil || *call.chapterID != "ch-1" {
		t.Errorf("ref 回指不对: refID=%v chapterID=%v", call.refID, call.chapterID)
	}
	if len(call.chunks) < 2 {
		t.Errorf("超长正文应切成多块，实际 %d 块", len(call.chunks))
	}
}

func TestIndexRefUnsupportedFallsBackToFullRebuild(t *testing.T) {
	store := &fakeChunkStore{}
	originals := &fakeOriginalReader{all: []domain.OriginalChapter{
		{ID: "oc-1", Content: "第一章正文"}, {ID: "oc-2", Content: "第二章正文"},
	}}
	svc := NewIndexService(store, fakeOwnerResolver{id: "owner-1"}, originals, &fakeCreativeChapterReader{}, nil, nil)

	if _, err := svc.IndexRef(context.Background(), domain.WorkKindOriginal, "ow-1", domain.ChunkRefChapter, "oc-1"); err != nil {
		t.Fatalf("回退全量重建失败: %v", err)
	}
	if originals.listCall != 1 {
		t.Errorf("不支持的 ref 组合应回退全量重建，实际全量读取 %d 次", originals.listCall)
	}
	if len(store.calls) != 2 {
		t.Errorf("全量重建应写 2 章，实际 %d", len(store.calls))
	}
}

func TestIndexRefClearsChunksWhenChapterGone(t *testing.T) {
	store := &fakeChunkStore{}
	reader := &fakeCreativeChapterReader{err: domain.ErrCreativeChapterNotFound}
	svc := NewIndexService(store, fakeOwnerResolver{id: "owner-1"}, &fakeOriginalReader{}, reader, nil, nil)

	if _, err := svc.IndexRef(context.Background(), domain.WorkKindCreative, "cw-1", domain.ChunkRefChapter, "ch-x"); err != nil {
		t.Fatalf("章节已删除时不该报错（应清空它的旧块）: %v", err)
	}
	if len(store.calls) != 1 || len(store.calls[0].chunks) != 0 {
		t.Fatalf("应清空该 ref 的旧块，实际 %#v", store.calls)
	}
}

func TestIndexRefPropagatesUnexpectedError(t *testing.T) {
	store := &fakeChunkStore{}
	reader := &fakeCreativeChapterReader{err: errors.New("数据库炸了")}
	svc := NewIndexService(store, fakeOwnerResolver{id: "owner-1"}, &fakeOriginalReader{}, reader, nil, nil)

	if _, err := svc.IndexRef(context.Background(), domain.WorkKindCreative, "cw-1", domain.ChunkRefChapter, "ch-1"); err == nil {
		t.Fatal("非“章节不存在”的错误应向上抛，交给任务重试")
	}
	if len(store.calls) != 0 {
		t.Errorf("出错时不该写分块，实际 %d 次", len(store.calls))
	}
}

func TestIndexDedupCollapsesBursts(t *testing.T) {
	d := newIndexDedup(5 * time.Second)
	base := time.Now()
	key := "creative|cw-1|chapter|ch-1"

	if !d.allow(key, base) {
		t.Fatal("首次应允许入队")
	}
	if d.allow(key, base.Add(1*time.Second)) {
		t.Error("窗口内的重复触发应被折叠（编辑器 1.5s 自动保存不该灌满任务表）")
	}
	if !d.allow(key, base.Add(6*time.Second)) {
		t.Error("超出窗口后应允许再次入队")
	}
	if !d.allow("creative|cw-1|chapter|ch-2", base.Add(6*time.Second)) {
		t.Error("不同来源互不影响")
	}
}

type fakeIndexTrigger struct {
	calls []string
	err   error
}

func (f *fakeIndexTrigger) EnqueueIndex(_ context.Context, workKind, workID, refKind, refID string) error {
	f.calls = append(f.calls, workKind+"/"+workID+"/"+refKind+"/"+refID)
	return f.err
}

func TestUpdateChapterTriggersIndexOnlyWhenContentChanged(t *testing.T) {
	chapter := &domain.CreativeChapter{ID: "ch-1", CreativeWorkID: "cw-1", Title: "开局", ChapterNo: 1, Content: "旧正文"}
	repo := &stubWritingRepo{chapter: chapter}
	svc := NewWritingService(repo, &stubCreativeReader{}, &stubContextReader{})
	trigger := &fakeIndexTrigger{}
	svc.SetIndexTrigger(trigger)

	// 只改标题：不该重建索引
	title := "新标题"
	if _, _, err := svc.UpdateChapter(context.Background(), "ch-1", UpdateChapterInput{Title: &title}); err != nil {
		t.Fatalf("改标题失败: %v", err)
	}
	if len(trigger.calls) != 0 {
		t.Fatalf("正文没变不该重建索引，实际 %v", trigger.calls)
	}

	// 改正文：应重建本章索引
	content := "新正文"
	if _, _, err := svc.UpdateChapter(context.Background(), "ch-1", UpdateChapterInput{Content: &content}); err != nil {
		t.Fatalf("改正文失败: %v", err)
	}
	if len(trigger.calls) != 1 || trigger.calls[0] != "creative/cw-1/chapter/ch-1" {
		t.Fatalf("应按章节重建索引，实际 %v", trigger.calls)
	}
}

func TestIndexTriggerFailureDoesNotBreakSaving(t *testing.T) {
	chapter := &domain.CreativeChapter{ID: "ch-2", CreativeWorkID: "cw-1", Title: "第二章", ChapterNo: 2, Content: "旧"}
	repo := &stubWritingRepo{chapter: chapter}
	svc := NewWritingService(repo, &stubCreativeReader{}, &stubContextReader{})
	svc.SetIndexTrigger(&fakeIndexTrigger{err: errors.New("任务队列不可用")})

	content := "新"
	if _, _, err := svc.UpdateChapter(context.Background(), "ch-2", UpdateChapterInput{Content: &content}); err != nil {
		t.Fatalf("索引入队失败不该让保存失败（索引是增强能力，不是写作的单点故障）: %v", err)
	}
}

// fakeOriginalRepo 只实现 Import 会用到的方法（其余由嵌入接口兜底）。
type fakeOriginalRepo struct {
	OriginalRepository
	work     *domain.OriginalWork
	replaced int
}

func (f *fakeOriginalRepo) GetWorkByID(context.Context, string) (*domain.OriginalWork, error) {
	return f.work, nil
}

func (f *fakeOriginalRepo) CreateFile(context.Context, *domain.UploadedFile) error { return nil }

func (f *fakeOriginalRepo) ReplaceChapters(context.Context, string, []domain.OriginalChapter, int64) error {
	f.replaced++
	return nil
}

func (f *fakeOriginalRepo) SetWorkSource(context.Context, string, domain.SourceType, string) error {
	return nil
}

func TestImportTriggersFullOriginalIndex(t *testing.T) {
	repo := &fakeOriginalRepo{work: &domain.OriginalWork{ID: "ow-1", ProjectID: "p-1", Title: "测试书"}}
	store, err := storage.NewLocalStore(t.TempDir())
	if err != nil {
		t.Fatalf("本地存储初始化失败: %v", err)
	}
	svc := NewOriginalService(repo, nil, store, 4<<20)
	trigger := &fakeIndexTrigger{}
	svc.SetIndexTrigger(trigger)

	book := "第1章 起\n\n梅雨下了整夜。\n\n第2章 归\n\n他推开门。\n"
	if _, err := svc.Import(context.Background(), "ow-1", "book.txt", strings.NewReader(book)); err != nil {
		t.Fatalf("导入失败: %v", err)
	}
	if repo.replaced != 1 {
		t.Fatalf("章节应整体替换一次，实际 %d", repo.replaced)
	}
	if len(trigger.calls) != 1 || trigger.calls[0] != "original/ow-1//" {
		t.Fatalf("导入完成应入队全量重建原著索引，实际 %v", trigger.calls)
	}
}
