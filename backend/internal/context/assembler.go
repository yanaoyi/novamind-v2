package context

import (
	"fmt"
	"strings"
)

// ChapterFacts 是组装上下文所需的"既有事实"（由写作服务提供）。
type ChapterFacts struct {
	ChapterID        string
	ChapterGoal      string
	CharacterContext string
	WorldContext     string
	TimelineContext  string
	PreviousContext  string
}

// RetrievalHit 是一条检索命中（§9.1 的产物，进上下文用）。
type RetrievalHit struct {
	ChunkID string
	RefKind string
	RefID   *string
	Seq     int
	Content string
	Score   float64
}

// BuildSections 把事实 + 检索 + 作者指令拼成 8 段。
//
// 之所以单独一个纯函数：组装逻辑（谁进哪一段、检索结果怎么标注来源）
// 与"从库里取数"分开，前者可以脱离数据库单测，后者留给服务层。
func BuildSections(facts ChapterFacts, originalHits, creativeHits []RetrievalHit, instruction string) Sections {
	return Sections{
		ChapterGoal:       facts.ChapterGoal,
		Characters:        facts.CharacterContext,
		World:             facts.WorldContext,
		Timeline:          facts.TimelineContext,
		PrevSummary:       facts.PreviousContext,
		RetrievedOriginal: FormatRetrievalWithin(originalHits, BudgetRetrievalOriginal),
		RetrievedCreative: FormatRetrievalWithin(creativeHits, BudgetRetrievalCreative),
		AuthorInstruction: instruction,
	}
}

// FormatRetrieval 把检索命中格式化成提示词里的一段文本。
//
// 每条都带上来源标注（ref_kind + 序号 + 相似度），这样：
//  1. 模型知道这段是"原文摘录/设定/记忆"而不是作者指令；
//  2. 作者事后能在 snapshot 里追到"这条结论出自哪一块"。
func FormatRetrieval(hits []RetrievalHit) string {
	return FormatRetrievalWithin(hits, 0)
}

// FormatRetrievalWithin 在 token 预算内拼检索段：**整条纳入，不切半句**。
//
// 为什么不在拼完之后再按 token 截断（那是 Assemble 的兜底行为）：
// 检索命中是"一条一条"的，按字符截断会留下半句话，模型可能把残句当成完整设定。
// 这里按条累加，放不下就整条丢弃，并在末尾如实说明丢了几条，
// 让作者在快照里看得出"预算吃满导致少给了几条"。
//
// maxTokens<=0 表示不限（FormatRetrieval 的语义，供单测与调试用）。
func FormatRetrievalWithin(hits []RetrievalHit, maxTokens int) string {
	if len(hits) == 0 {
		return ""
	}
	var sb strings.Builder
	used := 0
	included := 0
	for i, h := range hits {
		line := hitLine(i+1, h)
		cost := EstimateTokens(line) + 1 // +1：换行也占一点预算，别精确到把最后一条撑爆
		// 第一条永远纳入：命中本身有价值，预算再紧也不该给出空段。
		if included > 0 && maxTokens > 0 && used+cost > maxTokens {
			break
		}
		sb.WriteString(line)
		sb.WriteByte('\n')
		used += cost
		included++
	}
	if dropped := len(hits) - included; dropped > 0 {
		fmt.Fprintf(&sb, "（另有 %d 条命中因预算未纳入）\n", dropped)
	}
	return strings.TrimSpace(sb.String())
}

// hitLine 是单条命中在提示词里的呈现：来源标注 + 相关度 + 正文。
func hitLine(no int, h RetrievalHit) string {
	label := h.RefKind
	if h.Seq > 0 {
		label = fmt.Sprintf("%s#%d", label, h.Seq)
	}
	return fmt.Sprintf("【%d｜%s｜相关度 %.2f】%s", no, label, h.Score, strings.TrimSpace(h.Content))
}

// AssembleForChapter 是"事实 + 检索 → 预算内 8 段"的一步到位入口。
func AssembleForChapter(
	facts ChapterFacts,
	originalHits, creativeHits []RetrievalHit,
	instruction string,
) *Assembly {
	return Assemble(BuildSections(facts, originalHits, creativeHits, instruction))
}

// RetrievalSources 把命中整理成快照里的"检索来源"清单（P0-3 要求可追溯）。
//
// work_kind 由调用方给定（原著 / 二创），因为一条 chunk 的归属只有调用方知道
// （同一批 hits 可能来自两部作品的两条检索路）。
func RetrievalSources(workKind string, hits []RetrievalHit) []map[string]any {
	out := make([]map[string]any, 0, len(hits))
	for _, h := range hits {
		item := map[string]any{
			"chunk_id":  h.ChunkID,
			"work_kind": workKind,
			"ref_kind":  h.RefKind,
			"seq":       h.Seq,
			"score":     h.Score,
		}
		if h.RefID != nil {
			item["ref_id"] = *h.RefID
		}
		out = append(out, item)
	}
	return out
}
