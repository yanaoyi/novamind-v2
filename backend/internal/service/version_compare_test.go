package service

import (
	"testing"
)

func TestDiffPayloadReportsAddedRemovedChanged(t *testing.T) {
	before := map[string]any{
		"name":       "林砚",
		"importance": float64(4),
		"dna": map[string]any{
			"personality": map[string]any{"text": "克制", "weight": float64(80)},
			"ability":     map[string]any{"text": "剑术", "weight": float64(60)},
		},
	}
	after := map[string]any{
		"name":       "林砚",
		"importance": float64(5),
		"dna": map[string]any{
			"personality": map[string]any{"text": "克制", "weight": float64(80)},
			"ability":     map[string]any{"text": "剑术", "weight": float64(30)},
			"background":  map[string]any{"text": "边军出身", "weight": float64(100)},
		},
	}

	changes, summary := diffPayload(before, after)

	// 相同：name、dna.personality.text、dna.personality.weight、dna.ability.text
	if summary.Same != 4 {
		t.Fatalf("相同字段数应为 4，实际 %d", summary.Same)
	}
	// 变化：importance、dna.ability.weight
	if summary.Changed != 2 {
		t.Fatalf("变化字段数应为 2（importance + dna.ability.weight），实际 %d（%+v）", summary.Changed, changes)
	}
	// 新增：dna.background.text、dna.background.weight
	if summary.Added != 2 {
		t.Fatalf("新增字段数应为 2（dna.background.*），实际 %d（%+v）", summary.Added, changes)
	}

	// importance 4 → 5 属于变化，必须报出来
	found := false
	for _, ch := range changes {
		if ch.Path == "importance" && ch.Kind == "changed" && ch.Before == float64(4) && ch.After == float64(5) {
			found = true
		}
	}
	if !found {
		t.Fatalf("未报出 importance 的变化：%+v", changes)
	}
}

func TestDiffPayloadReportsRemovedField(t *testing.T) {
	before := map[string]any{"name": "A", "note": "旧备注"}
	after := map[string]any{"name": "A"}

	changes, summary := diffPayload(before, after)
	if summary.Removed != 1 {
		t.Fatalf("应报出 1 个删除字段，实际 %d（%+v）", summary.Removed, changes)
	}
	for _, ch := range changes {
		if ch.Path == "note" && ch.Kind == "removed" && ch.Before == "旧备注" && ch.After == nil {
			return
		}
	}
	t.Fatalf("未报出 note 的删除：%+v", changes)
}

func TestDiffLinesMarksChangedParagraph(t *testing.T) {
	before := "第一段\n第二段\n第三段"
	after := "第一段\n第二段改了\n第三段"

	lines, summary := DiffLines(before, after)

	if summary.Same != 2 || summary.Removed != 1 || summary.Added != 1 {
		t.Fatalf("行级统计不对：same=%d removed=%d added=%d", summary.Same, summary.Removed, summary.Added)
	}
	if lines[0].Kind != "same" || lines[0].Text != "第一段" {
		t.Fatalf("首行应为相同行：%+v", lines[0])
	}
	var sawRemoved, sawAdded bool
	for _, l := range lines {
		if l.Kind == "removed" && l.Text == "第二段" && l.OldNo == 2 {
			sawRemoved = true
		}
		if l.Kind == "added" && l.Text == "第二段改了" && l.NewNo == 2 {
			sawAdded = true
		}
	}
	if !sawRemoved || !sawAdded {
		t.Fatalf("未报出第二段的替换：%+v", lines)
	}
}

func TestDiffLinesHandlesEmptyAndCRLF(t *testing.T) {
	lines, summary := DiffLines("", "")
	if len(lines) != 0 || summary.Same != 0 {
		t.Fatalf("空正文应无差异：%+v", lines)
	}

	lines, summary = DiffLines("a\r\nb", "a\nb")
	if len(lines) != 2 || summary.Same != 2 {
		t.Fatalf("CRLF 与 LF 应当视为相同内容：%+v", lines)
	}
}
