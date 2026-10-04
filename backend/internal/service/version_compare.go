package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/yanaoyi/novamindv2/backend/internal/domain"
)

// 版本比较（规格书 §59：版本管理要支持「查看 / 恢复 / 比较」）。
//
// 比较分两种形态：
//   - 结构化实体（人物 / 世界观 / 大纲）：逐字段 diff，输出「字段路径 → 变更前后」；
//   - 章节：正文按行 diff，输出 added / removed / same 序列。
//
// 约定：版本号传 0 表示「当前状态」（不入库，现算一份快照用于比较）。

// ValueChange 是一个字段的变化。
type ValueChange struct {
	Path   string `json:"path"`
	Kind   string `json:"kind"` // added | removed | changed
	Before any    `json:"before"`
	After  any    `json:"after"`
}

// TextLine 是正文行级差异的一行。
type TextLine struct {
	Kind  string `json:"kind"` // same | added | removed
	OldNo int    `json:"old_no,omitempty"`
	NewNo int    `json:"new_no,omitempty"`
	Text  string `json:"text"`
}

// CompareSummary 是差异统计。
type CompareSummary struct {
	Added   int `json:"added"`
	Removed int `json:"removed"`
	Changed int `json:"changed"`
	Same    int `json:"same"`
}

// VersionCompare 是两个版本之间的差异结果。
type VersionCompare struct {
	EntityType string         `json:"entity_type"`
	EntityID   string         `json:"entity_id"`
	From       int            `json:"from"`
	To         int            `json:"to"`
	FromNote   string         `json:"from_note,omitempty"`
	ToNote     string         `json:"to_note,omitempty"`
	Changes    []ValueChange  `json:"changes"`
	Lines      []TextLine     `json:"lines"`
	Summary    CompareSummary `json:"summary"`
}

// Compare 比较结构化实体（人物 / 世界观 / 大纲）的两个版本。
func (s *VersionService) Compare(ctx context.Context, t, entityID string, fromNo, toNo int) (*VersionCompare, error) {
	kind, err := parseVersionType(t)
	if err != nil {
		return nil, err
	}
	if fromNo < 0 || toNo < 0 {
		return nil, fmt.Errorf("%w: 版本号必须 ≥ 0（0 表示当前状态）", domain.ErrVersionNoInvalid)
	}
	if fromNo == 0 && toNo == 0 {
		return nil, fmt.Errorf("%w: 两个版本号不能同时为当前状态", domain.ErrVersionNoInvalid)
	}

	beforePayload, beforeNote, err := s.entityPayload(ctx, kind, entityID, fromNo)
	if err != nil {
		return nil, err
	}
	afterPayload, afterNote, err := s.entityPayload(ctx, kind, entityID, toNo)
	if err != nil {
		return nil, err
	}

	result := &VersionCompare{
		EntityType: string(kind), EntityID: entityID, From: fromNo, To: toNo,
		FromNote: beforeNote, ToNote: afterNote,
		Changes: []ValueChange{}, Lines: []TextLine{},
	}
	result.Changes, result.Summary = diffPayload(beforePayload, afterPayload)
	return result, nil
}

// entityPayload 取某个版本的快照；no=0 时现算一份「当前状态」的快照（不落库）。
func (s *VersionService) entityPayload(ctx context.Context, kind domain.EntityVersionType, entityID string, no int) (map[string]any, string, error) {
	if no > 0 {
		version, err := s.repo.GetByNo(ctx, kind, entityID, no)
		if err != nil {
			return nil, "", err
		}
		return version.Payload, version.Note, nil
	}
	switch kind {
	case domain.VersionCreativeCharacter:
		payload, err := s.buildCharacterPayload(ctx, entityID)
		return payload, "当前状态", err
	case domain.VersionCreativeWorld:
		payload, err := s.buildWorldPayload(ctx, entityID)
		return payload, "当前状态", err
	case domain.VersionCreativeOutline:
		payload, err := s.buildOutlinePayload(ctx, entityID)
		return payload, "当前状态", err
	case domain.VersionCreativeOutlineTree:
		if s.outlineTree == nil {
			return nil, "", fmt.Errorf("%w: %s", domain.ErrVersionTypeBad, kind)
		}
		payload, err := s.outlineTree.TreePayload(ctx, entityID)
		return payload, "当前状态", err
	default:
		return nil, "", fmt.Errorf("%w: %s", domain.ErrVersionTypeBad, kind)
	}
}

// CompareChapter 比较章节正文的两个版本（0 = 当前正文）。
func (s *VersionService) CompareChapter(ctx context.Context, chapterID string, fromNo, toNo int) (*VersionCompare, error) {
	before, beforeNote, err := s.chapterText(ctx, chapterID, fromNo)
	if err != nil {
		return nil, err
	}
	after, afterNote, err := s.chapterText(ctx, chapterID, toNo)
	if err != nil {
		return nil, err
	}

	lines, summary := DiffLines(before, after)
	return &VersionCompare{
		EntityType: "chapter", EntityID: chapterID, From: fromNo, To: toNo,
		FromNote: beforeNote, ToNote: afterNote,
		Changes: []ValueChange{},
		Lines:   lines,
		Summary: summary,
	}, nil
}

func (s *VersionService) chapterText(ctx context.Context, chapterID string, no int) (string, string, error) {
	if no == 0 {
		chapter, err := s.writing.GetChapter(ctx, chapterID)
		if err != nil {
			return "", "", err
		}
		return chapter.Content, "当前正文", nil
	}
	version, err := s.writing.GetVersion(ctx, chapterID, no)
	if err != nil {
		return "", "", err
	}
	return version.Content, version.Note, nil
}

// ---------- 结构化 diff ----------

// diffPayload 把两份快照拍平成「路径 → 值」再逐键比较。
func diffPayload(before, after map[string]any) ([]ValueChange, CompareSummary) {
	flatBefore := map[string]any{}
	flatAfter := map[string]any{}
	flattenJSON("", before, flatBefore)
	flattenJSON("", after, flatAfter)

	keys := map[string]struct{}{}
	for k := range flatBefore {
		keys[k] = struct{}{}
	}
	for k := range flatAfter {
		keys[k] = struct{}{}
	}
	sorted := make([]string, 0, len(keys))
	for k := range keys {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)

	changes := make([]ValueChange, 0)
	summary := CompareSummary{}
	for _, k := range sorted {
		b, hasB := flatBefore[k]
		a, hasA := flatAfter[k]
		switch {
		case hasB && !hasA:
			summary.Removed++
			changes = append(changes, ValueChange{Path: k, Kind: "removed", Before: b, After: nil})
		case !hasB && hasA:
			summary.Added++
			changes = append(changes, ValueChange{Path: k, Kind: "added", Before: nil, After: a})
		case !equalJSONValue(b, a):
			summary.Changed++
			changes = append(changes, ValueChange{Path: k, Kind: "changed", Before: b, After: a})
		default:
			summary.Same++
		}
	}
	return changes, summary
}

// flattenJSON 把嵌套结构拍平成 "a.b[0].c" 形状的路径。
// 对象数组优先用元素里的 id 做路径（可读性好，也能对齐"同一个对象改了什么"）。
func flattenJSON(prefix string, value any, out map[string]any) {
	switch v := value.(type) {
	case map[string]any:
		if len(v) == 0 {
			out[prefix] = map[string]any{}
			return
		}
		for key, child := range v {
			flattenJSON(joinPath(prefix, key), child, out)
		}
	case []any:
		if len(v) == 0 {
			out[prefix] = []any{}
			return
		}
		for i, child := range v {
			flattenJSON(fmt.Sprintf("%s[%d]", prefix, i), child, out)
		}
	default:
		out[prefix] = v
	}
}

func joinPath(prefix, key string) string {
	if prefix == "" {
		return key
	}
	return prefix + "." + key
}

func equalJSONValue(a, b any) bool {
	// JSON 解码后数字都是 float64，直接比较即可；结构用序列化兜底保证稳妥
	ra, errA := json.Marshal(a)
	rb, errB := json.Marshal(b)
	if errA != nil || errB != nil {
		return false
	}
	return string(ra) == string(rb)
}

// ---------- 正文行级 diff ----------

// maxDiffCells 限制 LCS 规模：超过就用「全删全增」的粗粒度结果，避免大长篇把 CPU 打满。
const maxDiffCells = 4_000_000

// DiffLines 对两段正文做行级差异（LCS）。
func DiffLines(before, after string) ([]TextLine, CompareSummary) {
	a := splitLines(before)
	b := splitLines(after)

	// 先掐掉公共前后缀，能让绝大多数"只改了几段"的章节 diff 变得很小
	head := 0
	for head < len(a) && head < len(b) && a[head] == b[head] {
		head++
	}
	tail := 0
	for tail < len(a)-head && tail < len(b)-head && a[len(a)-1-tail] == b[len(b)-1-tail] {
		tail++
	}

	midA := a[head : len(a)-tail]
	midB := b[head : len(b)-tail]

	lines := make([]TextLine, 0, len(a)+len(b))
	summary := CompareSummary{}
	for i := 0; i < head; i++ {
		summary.Same++
		lines = append(lines, TextLine{Kind: "same", OldNo: i + 1, NewNo: i + 1, Text: a[i]})
	}

	if len(midA)*len(midB) > maxDiffCells {
		for i, text := range midA {
			summary.Removed++
			lines = append(lines, TextLine{Kind: "removed", OldNo: head + i + 1, Text: text})
		}
		for i, text := range midB {
			summary.Added++
			lines = append(lines, TextLine{Kind: "added", NewNo: head + i + 1, Text: text})
		}
	} else {
		ops := lcsOps(midA, midB)
		for _, op := range ops {
			switch op.kind {
			case "same":
				summary.Same++
				lines = append(lines, TextLine{Kind: "same", OldNo: head + op.oldNo + 1, NewNo: head + op.newNo + 1, Text: op.text})
			case "removed":
				summary.Removed++
				lines = append(lines, TextLine{Kind: "removed", OldNo: head + op.oldNo + 1, Text: op.text})
			case "added":
				summary.Added++
				lines = append(lines, TextLine{Kind: "added", NewNo: head + op.newNo + 1, Text: op.text})
			}
		}
	}

	for i := 0; i < tail; i++ {
		oldNo := len(a) - tail + i + 1
		newNo := len(b) - tail + i + 1
		summary.Same++
		lines = append(lines, TextLine{Kind: "same", OldNo: oldNo, NewNo: newNo, Text: a[oldNo-1]})
	}
	return lines, summary
}

type diffOp struct {
	kind  string
	text  string
	oldNo int // 0-based
	newNo int
}

func lcsOps(a, b []string) []diffOp {
	// dp[i][j] = a[i:] 与 b[j:] 的最长公共子序列长度
	dp := make([][]int, len(a)+1)
	for i := range dp {
		dp[i] = make([]int, len(b)+1)
	}
	for i := len(a) - 1; i >= 0; i-- {
		for j := len(b) - 1; j >= 0; j-- {
			if a[i] == b[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else if dp[i+1][j] >= dp[i][j+1] {
				dp[i][j] = dp[i+1][j]
			} else {
				dp[i][j] = dp[i][j+1]
			}
		}
	}

	ops := make([]diffOp, 0, len(a)+len(b))
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		switch {
		case a[i] == b[j]:
			ops = append(ops, diffOp{kind: "same", text: a[i], oldNo: i, newNo: j})
			i++
			j++
		case dp[i+1][j] >= dp[i][j+1]:
			ops = append(ops, diffOp{kind: "removed", text: a[i], oldNo: i})
			i++
		default:
			ops = append(ops, diffOp{kind: "added", text: b[j], newNo: j})
			j++
		}
	}
	for ; i < len(a); i++ {
		ops = append(ops, diffOp{kind: "removed", text: a[i], oldNo: i})
	}
	for ; j < len(b); j++ {
		ops = append(ops, diffOp{kind: "added", text: b[j], newNo: j})
	}
	return ops
}

func splitLines(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}
