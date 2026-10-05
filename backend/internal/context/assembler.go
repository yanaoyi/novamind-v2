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
		RetrievedOriginal: FormatRetrieval(originalHits),
		RetrievedCreative: FormatRetrieval(creativeHits),
		AuthorInstruction: instruction,
	}
}

// FormatRetrieval 把检索命中格式化成提示词里的一段文本。
//
// 每条都带上来源标注（ref_kind + 序号 + 相似度），这样：
//  1. 模型知道这段是"原文摘录/设定/记忆"而不是作者指令；
//  2. 作者事后能在 snapshot 里追到"这条结论出自哪一块"。
func FormatRetrieval(hits []RetrievalHit) string {
	if len(hits) == 0 {
		return ""
	}
	var sb strings.Builder
	for i, h := range hits {
		label := h.RefKind
		if h.Seq > 0 {
			label = fmt.Sprintf("%s#%d", label, h.Seq)
		}
		fmt.Fprintf(&sb, "【%d｜%s｜相关度 %.2f】%s\n", i+1, label, h.Score, strings.TrimSpace(h.Content))
	}
	return strings.TrimSpace(sb.String())
}

// AssembleForChapter 是"事实 + 检索 → 预算内 8 段"的一步到位入口。
func AssembleForChapter(
	facts ChapterFacts,
	originalHits, creativeHits []RetrievalHit,
	instruction string,
) *Assembly {
	return Assemble(BuildSections(facts, originalHits, creativeHits, instruction))
}
