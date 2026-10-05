package context

import (
	"strings"
	"testing"
)

func TestBuildSectionsMapsSources(t *testing.T) {
	facts := ChapterFacts{
		ChapterID: "ch-1", ChapterGoal: "让沈砚发现账册异常",
		CharacterContext: "- 沈砚（INHERITED）：克制",
		WorldContext:     "世界：江城",
		TimelineContext:  "第3章：灯会",
		PreviousContext:  "第2章 对峙：与督军的人结怨",
	}
	original := []RetrievalHit{{ChunkID: "c1", RefKind: "chapter", Seq: 1, Content: "第一章埋下青铜钥匙。", Score: 0.9}}
	creative := []RetrievalHit{{ChunkID: "c2", RefKind: "chapter", Seq: 1, Content: "同人里钥匙改成了铜扣。", Score: 0.7}}

	got := BuildSections(facts, original, creative, "写 800 字")
	if got.ChapterGoal != facts.ChapterGoal || got.Characters != facts.CharacterContext {
		t.Error("固定段应原样带入")
	}
	if !strings.Contains(got.RetrievedOriginal, "青铜钥匙") || !strings.Contains(got.RetrievedCreative, "铜扣") {
		t.Errorf("检索段未正确带入: %q / %q", got.RetrievedOriginal, got.RetrievedCreative)
	}
	if got.AuthorInstruction != "写 800 字" {
		t.Errorf("作者指令未带入: %q", got.AuthorInstruction)
	}
}

func TestFormatRetrievalLabelsSourceAndScore(t *testing.T) {
	if FormatRetrieval(nil) != "" {
		t.Error("无命中应返回空串（不占预算）")
	}
	text := FormatRetrieval([]RetrievalHit{
		{RefKind: "chapter", Seq: 2, Content: "正文片段", Score: 0.8123},
		{RefKind: "memory_fact", Content: "沈砚左臂受伤", Score: 0.5},
	})
	if !strings.Contains(text, "chapter#2") || !strings.Contains(text, "0.81") {
		t.Errorf("应标注来源与相关度，实际: %s", text)
	}
	if !strings.Contains(text, "memory_fact") {
		t.Errorf("记忆类来源也应标注，实际: %s", text)
	}
}

func TestAssembleForChapterRespectsBudget(t *testing.T) {
	facts := ChapterFacts{ChapterGoal: text(100), CharacterContext: text(100), WorldContext: text(100)}
	hits := make([]RetrievalHit, 0, 8)
	for i := 0; i < 8; i++ {
		hits = append(hits, RetrievalHit{
			ChunkID: "c", RefKind: "chapter", Seq: i,
			Content: text(400), Score: 0.9,
		})
	}
	got := AssembleForChapter(facts, hits, nil, text(100))
	if got.Tokens["retrieved_original"] > BudgetRetrievalOriginal {
		t.Errorf("检索段应受自身预算约束，实际 %d", got.Tokens["retrieved_original"])
	}
	if got.Total > TotalBudget {
		t.Errorf("总量不应超预算，实际 %d", got.Total)
	}
	if got.Tokens["chapter_goal"] == 0 || got.Tokens["characters"] == 0 {
		t.Error("固定段不该被清空")
	}
}

// 检索段必须"整条纳入"：预算不够时丢整条并说明，而不是把某条截成半句话
// （半句话会被模型当成完整设定，这比少给一条更危险）。
func TestFormatRetrievalWithinDropsWholeHits(t *testing.T) {
	hits := make([]RetrievalHit, 0, 8)
	markers := []string{}
	for i := 0; i < 8; i++ {
		marker := strings.Repeat(string(rune('甲'+i)), 500)
		markers = append(markers, marker)
		hits = append(hits, RetrievalHit{
			ChunkID: "c", RefKind: "chapter", Seq: i + 1, Content: marker, Score: 0.5,
		})
	}
	got := FormatRetrievalWithin(hits, BudgetRetrievalOriginal)

	included := strings.Count(got, "【")
	if included == 0 || included >= len(hits) {
		t.Fatalf("应丢弃后面的整条命中，实际纳入 %d 条", included)
	}
	if !strings.Contains(got, "因预算未纳入") {
		t.Errorf("丢弃命中时应如实标注，实际: %s", got)
	}
	if EstimateTokens(got) > BudgetRetrievalOriginal {
		t.Errorf("含说明行也不该撑出预算，实际 %d", EstimateTokens(got))
	}
	full := 0
	for _, m := range markers {
		if strings.Contains(got, m) {
			full++
		}
	}
	if full != included {
		t.Errorf("纳入 %d 条但只有 %d 条正文完整（存在被切断的残句）", included, full)
	}
}

func TestRetrievalSourcesCarryWorkKindAndRef(t *testing.T) {
	refID := "ch-9"
	sources := RetrievalSources("creative", []RetrievalHit{
		{ChunkID: "c1", RefKind: "chapter", RefID: &refID, Seq: 2, Score: 0.31},
	})
	if len(sources) != 1 {
		t.Fatalf("应产出 1 条来源，实际 %d", len(sources))
	}
	item := sources[0]
	if item["work_kind"] != "creative" || item["ref_kind"] != "chapter" || item["ref_id"] != "ch-9" {
		t.Errorf("来源标注不完整: %#v", item)
	}
	if _, ok := item["score"].(float64); !ok {
		t.Errorf("score 应为数值以便快照检索: %#v", item["score"])
	}
}
