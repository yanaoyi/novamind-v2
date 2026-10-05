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
