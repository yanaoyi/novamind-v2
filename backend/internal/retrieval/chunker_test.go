package retrieval

import (
	"strings"
	"testing"
)

func TestChunkChapterEmptyAndShort(t *testing.T) {
	if got := ChunkChapter("   \n\n  "); got != nil {
		t.Fatalf("空内容应返回 nil，实际 %+v", got)
	}
	got := ChunkChapter("第一章 起\n\n沈砚回到家，发现门虚掩着。")
	if len(got) != 1 {
		t.Fatalf("短内容应整段一块，实际 %d 块", len(got))
	}
	if got[0].Seq != 0 || !strings.Contains(got[0].Content, "沈砚回到家") {
		t.Fatalf("短内容切块不正确: %+v", got[0])
	}
	if got[0].TokenCount <= 0 {
		t.Error("token 数应大于 0")
	}
}

func TestChunkChapterSplitsWithOverlap(t *testing.T) {
	// 造 3000 字、每 100 字一个换行的文本
	var sb strings.Builder
	for i := 0; i < 30; i++ {
		sb.WriteString(strings.Repeat("字", 100))
		sb.WriteString("\n")
	}
	text := sb.String()

	chunks := ChunkChapter(text)
	if len(chunks) < 3 {
		t.Fatalf("3000 字应切成多块，实际 %d 块", len(chunks))
	}
	for i, c := range chunks {
		if c.Seq != i {
			t.Errorf("第 %d 块的 Seq 应为 %d，实际 %d", i, i, c.Seq)
		}
		if n := len([]rune(c.Content)); n > MaxChunkRunes {
			t.Errorf("第 %d 块超过上限：%d 字", i, n)
		}
	}
	// 重叠：前一块的尾部内容应出现在后一块里（上下文不断档）
	prevTail := tailRunes(chunks[0].Content, 60)
	if prevTail != "" && !strings.Contains(chunks[1].Content, prevTail) {
		t.Errorf("相邻块之间缺少重叠：前块尾部 %q 未出现在后块中", prevTail)
	}
	// 覆盖：把各块拼起来应能覆盖原文的全部字符（允许重叠导致的重复）
	joined := ""
	for _, c := range chunks {
		joined += c.Content
	}
	if !strings.Contains(joined, "字字") {
		t.Error("切块后内容丢失")
	}
	total := 0
	for _, c := range chunks {
		total += len([]rune(c.Content))
	}
	if total < 3000 {
		t.Errorf("切块后总字数少于原文：%d", total)
	}
}

func TestChunkChapterPrefersParagraphBoundary(t *testing.T) {
	// 第 700 字处有换行：应在这里收口，而不是切到 800 字中间
	text := strings.Repeat("甲", 700) + "\n" + strings.Repeat("乙", 500)
	chunks := ChunkChapter(text)
	if len(chunks) < 2 {
		t.Fatalf("应切成至少 2 块，实际 %d", len(chunks))
	}
	if strings.Contains(chunks[0].Content, "乙") {
		t.Error("第一块不应越过分段边界把下一段也切进来")
	}
}

func TestChunkSingleLongContentStillSplits(t *testing.T) {
	if got := ChunkSingle("  "); got != nil {
		t.Fatalf("空内容应为 nil，实际 %+v", got)
	}
	short := ChunkSingle("主角：沈砚；状态：左臂受伤")
	if len(short) != 1 {
		t.Fatalf("单条内容应为一块，实际 %d 块", len(short))
	}
	long := ChunkSingle(strings.Repeat("规", 2000))
	if len(long) < 2 {
		t.Fatalf("超长单条也应被切开，实际 %d 块", len(long))
	}
}

func tailRunes(s string, n int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= n {
		return string(r)
	}
	return string(r[len(r)-n:])
}
