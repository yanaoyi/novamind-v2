package context

// 上下文预算（Phase 9 §9.2.1，任务书给定默认值）。
const (
	TotalBudget = 8000

	BudgetChapterGoal       = 800
	BudgetCharacters        = 1200
	BudgetWorld             = 800
	BudgetTimeline          = 600
	BudgetPrevSummary       = 600
	BudgetRetrievalOriginal = 1500
	BudgetRetrievalCreative = 1500
)

// Sections 是待组装的 8 段上下文。
type Sections struct {
	ChapterGoal string
	Characters  string
	World       string
	Timeline    string
	PrevSummary string
	// RetrievedOriginal / RetrievedCreative 来自检索（§9.1）
	RetrievedOriginal string
	RetrievedCreative string
	// AuthorInstruction 是作者指令，最后才截
	AuthorInstruction string
}

// Assembly 是组装结果：截断后的 8 段 + 每段 token 数 + 被截断的段落名。
type Assembly struct {
	Sections  Sections
	Tokens    map[string]int
	Truncated []string
	Total     int
}

// sectionSpec 描述一段在做预算时的行为。
type sectionSpec struct {
	name   string
	get    func(*Sections) string
	set    func(*Sections, string)
	budget int
	// fixed=true 表示"永不截断"（任务书：人物 / 世界 / 当前章节目标固定优先）
	fixed bool
	// order 是"总预算不够时"的截断顺序：数字越小越先被牺牲
	order int
}

// specs 的顺序与截断优先级一致（任务书：检索片段 → 前情摘要 → 时间线 → 作者指令）。
var specs = []sectionSpec{
	{name: "retrieved_original", get: func(s *Sections) string { return s.RetrievedOriginal },
		set: func(s *Sections, v string) { s.RetrievedOriginal = v }, budget: BudgetRetrievalOriginal, order: 1},
	{name: "retrieved_creative", get: func(s *Sections) string { return s.RetrievedCreative },
		set: func(s *Sections, v string) { s.RetrievedCreative = v }, budget: BudgetRetrievalCreative, order: 2},
	{name: "prev_summary", get: func(s *Sections) string { return s.PrevSummary },
		set: func(s *Sections, v string) { s.PrevSummary = v }, budget: BudgetPrevSummary, order: 3},
	{name: "timeline", get: func(s *Sections) string { return s.Timeline },
		set: func(s *Sections, v string) { s.Timeline = v }, budget: BudgetTimeline, order: 4},
	{name: "author_instruction", get: func(s *Sections) string { return s.AuthorInstruction },
		set: func(s *Sections, v string) { s.AuthorInstruction = v }, budget: TotalBudget, order: 5},
	// 固定段：不参与截断，也不被总预算裁剪（任务书明确要求）
	{name: "chapter_goal", get: func(s *Sections) string { return s.ChapterGoal },
		set: func(s *Sections, v string) { s.ChapterGoal = v }, budget: BudgetChapterGoal, fixed: true},
	{name: "characters", get: func(s *Sections) string { return s.Characters },
		set: func(s *Sections, v string) { s.Characters = v }, budget: BudgetCharacters, fixed: true},
	{name: "world", get: func(s *Sections) string { return s.World },
		set: func(s *Sections, v string) { s.World = v }, budget: BudgetWorld, fixed: true},
}

// Assemble 按预算组装上下文。
//
// 两步：
//  1. 各段先受**自己的**预算约束（固定段不受限）；
//  2. 若总量仍超 TotalBudget，按 order 依次把可截段砍到 0（作者指令最后被砍）。
//
// 固定段（当前章节目标 / 人物 DNA / 世界规则）在任何情况下都不截 ——
// 任务书的原话是"固定优先，不截"，宁可超预算也不能让 AI 丢掉人设与世界规则。
func Assemble(in Sections) *Assembly {
	out := Assembly{Sections: in, Tokens: map[string]int{}}

	// 第 1 步：各段受自身预算
	for _, spec := range specs {
		if spec.fixed {
			continue
		}
		text := spec.get(&out.Sections)
		if EstimateTokens(text) > spec.budget {
			text = truncateToTokens(text, spec.budget)
			spec.set(&out.Sections, text)
			out.Truncated = append(out.Truncated, spec.name)
		}
	}

	// 第 2 步：总预算不够时按优先级继续牺牲
	total := 0
	for _, spec := range specs {
		total += EstimateTokens(spec.get(&out.Sections))
	}
	for _, spec := range specs {
		if spec.fixed || total <= TotalBudget {
			continue
		}
		text := spec.get(&out.Sections)
		own := EstimateTokens(text)
		if own == 0 {
			continue
		}
		over := total - TotalBudget
		keep := own - over
		if keep < 0 {
			keep = 0
		}
		spec.set(&out.Sections, truncateToTokens(text, keep))
		now := EstimateTokens(spec.get(&out.Sections))
		total -= own - now
		if !contains(out.Truncated, spec.name) {
			out.Truncated = append(out.Truncated, spec.name)
		}
	}

	for _, spec := range specs {
		out.Tokens[spec.name] = EstimateTokens(spec.get(&out.Sections))
	}
	out.Total = total
	return &out
}

func contains(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}
