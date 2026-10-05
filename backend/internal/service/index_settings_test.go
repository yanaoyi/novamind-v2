package service

import (
	"context"
	"strings"
	"testing"

	"github.com/yanaoyi/novamindv2/backend/internal/domain"
)

// 这组测试锁的是 §9.1"其余来源"：人物 / 世界规则 / 大纲节点进索引，且删除后不留旧块。

type fakeSettingReader struct {
	characters []domain.CreativeCharacter
	detail     *WorldDetail
}

func (f *fakeSettingReader) ListCharacters(context.Context, string) ([]domain.CreativeCharacter, error) {
	return f.characters, nil
}

func (f *fakeSettingReader) GetWorldDetail(context.Context, string) (*WorldDetail, error) {
	return f.detail, nil
}

type fakeOutlineReader struct {
	outlines []domain.Outline
	nodes    map[string][]domain.OutlineNode
}

func (f *fakeOutlineReader) ListOutlines(context.Context, string) ([]domain.Outline, error) {
	return f.outlines, nil
}

func (f *fakeOutlineReader) ListNodes(_ context.Context, outlineID string) ([]domain.OutlineNode, error) {
	return f.nodes[outlineID], nil
}

func newSettingIndexService(chapter *domain.CreativeChapter) (*IndexService, *fakeChunkStore) {
	store := &fakeChunkStore{}
	settings := &fakeSettingReader{
		characters: []domain.CreativeCharacter{{
			ID: "cc-1", Name: "沈砚", Description: "克制，认死理", SourceType: domain.SourceModified, Importance: 4,
			DNA: domain.CharacterDNA{},
		}},
		detail: &WorldDetail{
			World: &domain.CreativeWorld{Name: "江城"},
			Rules: []domain.CreativeWorldRule{
				{ID: "cr-1", Status: domain.RuleInherited, Category: "社会", Name: "夜禁", Description: "子时后不得出城"},
				{ID: "cr-2", Status: domain.RuleRemoved, Category: "社会", Name: "已删规则", Description: "不该进索引"},
			},
		},
	}
	outlines := &fakeOutlineReader{
		outlines: []domain.Outline{{ID: "o-1", Title: "主大纲"}},
		nodes: map[string][]domain.OutlineNode{
			"o-1": {{ID: "on-1", Level: domain.OutlineLevelChapter, Title: "夜审账册", Conflict: "督军的人盯梢", Outcome: "账册到手"}},
		},
	}
	reader := &fakeCreativeChapterReader{all: []domain.CreativeChapter{*chapter}}
	svc := NewIndexService(store, fakeOwnerResolver{id: "owner-1"},
		&fakeOriginalReader{}, reader, settings, outlines)
	return svc, store
}

func TestIndexWorkIncludesSettingSources(t *testing.T) {
	chapter := &domain.CreativeChapter{ID: "ch-1", CreativeWorkID: "cw-1", Content: "正文"}
	svc, store := newSettingIndexService(chapter)

	result, err := svc.IndexWork(context.Background(), domain.WorkKindCreative, "cw-1")
	if err != nil {
		t.Fatalf("全量重建失败: %v", err)
	}
	// 1 章 + 1 人物 + 1 条有效规则（REMOVED 的不该进）+ 1 个大纲节点
	if result.Items != 4 {
		t.Errorf("应索引 4 个来源（含被移除的规则要排除），实际 %d（by_kind=%v）", result.Items, result.ByKind)
	}
	if result.ByKind[domain.ChunkRefCharacter] == 0 || result.ByKind[domain.ChunkRefWorldRule] == 0 ||
		result.ByKind[domain.ChunkRefOutlineNode] == 0 {
		t.Errorf("人物/规则/大纲节点都应进索引，实际 %v", result.ByKind)
	}
	joined := ""
	for _, call := range store.calls {
		for _, c := range call.chunks {
			joined += c.Content + "\n"
		}
	}
	for _, want := range []string{"沈砚", "夜禁", "夜审账册", "督军的人盯梢"} {
		if !strings.Contains(joined, want) {
			t.Errorf("索引文本应含 %q，实际：\n%s", want, joined)
		}
	}
	if strings.Contains(joined, "已删规则") {
		t.Error("已移除的规则不该进索引")
	}
}

// 全量重建必须等于"只保留现存来源"：剪枝调用要按来源类型带上本次的 ref 集合。
func TestIndexWorkPrunesStaleRefs(t *testing.T) {
	chapter := &domain.CreativeChapter{ID: "ch-1", CreativeWorkID: "cw-1", Content: "正文"}
	svc, store := newSettingIndexService(chapter)

	if _, err := svc.IndexWork(context.Background(), domain.WorkKindCreative, "cw-1"); err != nil {
		t.Fatalf("全量重建失败: %v", err)
	}
	pruned := map[string][]string{}
	for _, call := range store.prunes {
		pruned[call.refKind] = call.keep
	}
	for _, kind := range []string{
		domain.ChunkRefChapter, domain.ChunkRefCharacter,
		domain.ChunkRefWorldRule, domain.ChunkRefOutlineNode,
	} {
		if _, ok := pruned[kind]; !ok {
			t.Errorf("来源类型 %s 应被剪枝（否则删掉的来源会留在索引里）", kind)
		}
	}
	if len(pruned[domain.ChunkRefWorldRule]) != 1 || pruned[domain.ChunkRefWorldRule][0] != "cr-1" {
		t.Errorf("剪枝应保留现存规则 cr-1，实际 %v", pruned[domain.ChunkRefWorldRule])
	}
	// 记忆事实/摘要归 MemoryService 管，索引服务不许碰
	if _, ok := pruned[domain.ChunkRefMemoryFact]; ok {
		t.Error("索引服务不该剪记忆事实的块（那是 MemoryService 的地盘）")
	}
}

func TestIndexRefForSettingSources(t *testing.T) {
	chapter := &domain.CreativeChapter{ID: "ch-1", CreativeWorkID: "cw-1", Content: "正文"}
	cases := []struct {
		refKind string
		refID   string
		want    string
	}{
		{domain.ChunkRefCharacter, "cc-1", "沈砚"},
		{domain.ChunkRefWorldRule, "cr-1", "夜禁"},
		{domain.ChunkRefOutlineNode, "on-1", "夜审账册"},
	}
	for _, c := range cases {
		t.Run(c.refKind, func(t *testing.T) {
			svc, store := newSettingIndexService(chapter)
			if _, err := svc.IndexRef(context.Background(), domain.WorkKindCreative, "cw-1", c.refKind, c.refID); err != nil {
				t.Fatalf("增量索引失败: %v", err)
			}
			if len(store.calls) != 1 {
				t.Fatalf("只该写这一个来源，实际 %d 次", len(store.calls))
			}
			call := store.calls[0]
			if call.refKind != c.refKind || call.refID == nil || *call.refID != c.refID {
				t.Errorf("ref 归属不对: %#v", call)
			}
			if len(call.chunks) == 0 || !strings.Contains(call.chunks[0].Content, c.want) {
				t.Errorf("内容应含 %q，实际 %#v", c.want, call.chunks)
			}
		})
	}
}

// 来源已被删除（比如人物删了、规则移除成 REMOVED）时，要清空它的旧块。
func TestIndexRefClearsRemovedSettingSource(t *testing.T) {
	chapter := &domain.CreativeChapter{ID: "ch-1", CreativeWorkID: "cw-1", Content: "正文"}
	svc, store := newSettingIndexService(chapter)

	// cr-2 是 REMOVED，收集时会被跳过 → 增量重建等价于"清空它的块"
	if _, err := svc.IndexRef(context.Background(), domain.WorkKindCreative, "cw-1", domain.ChunkRefWorldRule, "cr-2"); err != nil {
		t.Fatalf("清理已移除规则失败: %v", err)
	}
	if len(store.calls) != 1 || len(store.calls[0].chunks) != 0 {
		t.Fatalf("应清空该 ref 的块，实际 %#v", store.calls)
	}
}

type fakeSettingServiceTrigger struct{ calls []string }

func (f *fakeSettingServiceTrigger) EnqueueIndex(_ context.Context, workKind, workID, refKind, refID string) error {
	f.calls = append(f.calls, workKind+"/"+workID+"/"+refKind+"/"+refID)
	return nil
}

// stubOutlineRepo 只实现本组测试用到的方法（其余由嵌入接口兜底）。
type stubOutlineRepo struct {
	OutlineRepository
	outline *domain.Outline
	created *domain.OutlineNode
}

func (r *stubOutlineRepo) GetOutline(context.Context, string) (*domain.Outline, error) {
	return r.outline, nil
}

func (r *stubOutlineRepo) CreateNodeLocked(_ context.Context, n *domain.OutlineNode) error {
	n.ID = "on-9"
	r.created = n
	return nil
}

func TestOutlineNodeCreateTriggersIndex(t *testing.T) {
	repo := &stubOutlineRepo{outline: &domain.Outline{ID: "o-1", CreativeWorkID: "cw-1"}}
	svc := NewOutlineService(repo, &stubCreativeReader{}, nil, nil)
	trigger := &fakeSettingServiceTrigger{}
	svc.SetIndexTrigger(trigger)

	node, err := svc.CreateNode(context.Background(), "o-1", CreateNodeInput{Title: "夜审账册"})
	if err != nil {
		t.Fatalf("新建大纲节点失败: %v", err)
	}
	if len(trigger.calls) != 1 || trigger.calls[0] != "creative/cw-1/outline_node/"+node.ID {
		t.Fatalf("新建节点应入队重建该节点索引，实际 %v", trigger.calls)
	}
}
