package context

import (
	"strings"
	"testing"
)

func text(tokens int) string {
	return strings.Repeat("字", runesForTokens(tokens))
}

func TestAssembleWithinBudgetKeepsEverything(t *testing.T) {
	in := Sections{
		ChapterGoal:       text(100),
		Characters:        text(100),
		World:             text(100),
		Timeline:          text(100),
		PrevSummary:       text(100),
		RetrievedOriginal: text(100),
		RetrievedCreative: text(100),
		AuthorInstruction: text(100),
	}
	got := Assemble(in)
	if len(got.Truncated) != 0 {
		t.Fatalf("都在预算内不该截断，实际截断 %v", got.Truncated)
	}
	if got.Total != 800 {
		t.Errorf("总量应为 800，实际 %d", got.Total)
	}
}

func TestAssembleCutsPerSectionBudgetFirst(t *testing.T) {
	// 检索段超自己的预算（1500），应被截到 1500；其它段不动
	in := Sections{
		ChapterGoal:       text(100),
		Characters:        text(100),
		RetrievedOriginal: text(3000),
		AuthorInstruction: text(100),
	}
	got := Assemble(in)
	if got.Tokens["retrieved_original"] != BudgetRetrievalOriginal {
		t.Errorf("检索段应被截到 %d，实际 %d", BudgetRetrievalOriginal, got.Tokens["retrieved_original"])
	}
	if got.Tokens["author_instruction"] != 100 {
		t.Errorf("作者指令不该被截，实际 %d", got.Tokens["author_instruction"])
	}
	if !contains(got.Truncated, "retrieved_original") {
		t.Errorf("应记录被截断的段，实际 %v", got.Truncated)
	}
}

func TestAssembleFixedSectionsNeverCut(t *testing.T) {
	// 固定段故意超预算（各 5000 tokens），只有可截段被牺牲，固定段一字不掉
	in := Sections{
		ChapterGoal:       text(5000),
		Characters:        text(5000),
		World:             text(5000),
		Timeline:          text(600),
		PrevSummary:       text(600),
		RetrievedOriginal: text(1500),
		RetrievedCreative: text(1500),
		AuthorInstruction: text(400),
	}
	got := Assemble(in)
	if got.Tokens["chapter_goal"] != 5000 || got.Tokens["characters"] != 5000 || got.Tokens["world"] != 5000 {
		t.Fatalf("固定段不该被截：%v", got.Tokens)
	}
	// 可截段按顺序被牺牲：检索先没，其次前情摘要，最后才是作者指令
	if got.Tokens["retrieved_original"] != 0 || got.Tokens["retrieved_creative"] != 0 {
		t.Errorf("检索段应最先被砍到 0，实际 %v/%v", got.Tokens["retrieved_original"], got.Tokens["retrieved_creative"])
	}
	if got.Tokens["prev_summary"] != 0 {
		t.Errorf("检索砍完仍超预算时应继续砍前情摘要，实际 %d", got.Tokens["prev_summary"])
	}
}

func TestAssembleAuthorInstructionCutLast(t *testing.T) {
	// 总量刚好需要砍一点点：只允许动检索段，作者指令必须完好
	in := Sections{
		ChapterGoal:       text(800),
		Characters:        text(1200),
		World:             text(800),
		RetrievedOriginal: text(1500),
		RetrievedCreative: text(1500),
		AuthorInstruction: text(1000),
	}
	got := Assemble(in)
	if got.Tokens["author_instruction"] != 1000 {
		t.Fatalf("作者指令应最后才被截，实际 %d", got.Tokens["author_instruction"])
	}
	if got.Total > TotalBudget {
		t.Errorf("组装后总量不应超预算，实际 %d", got.Total)
	}
}
