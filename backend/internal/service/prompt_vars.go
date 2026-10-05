package service

// 内部上下文组装包与标准库 context 同名，按别名引入（ctxengine）。
import ctxengine "github.com/yanaoyi/novamindv2/backend/internal/context"

// 本文件集中放"服务传给 Prompt 模板的变量集"。
//
// 为什么单独一个文件：模板引擎是 missingkey=error（模板引用了变量而调用方没传 →
// 渲染直接失败）。2026-10-05 已经踩过一次 —— chapter_generate.v2 先进仓库、
// 调用方还没补新增的两个变量，结果"写本章"直接渲染失败，而当时的 e2e 是在引入
// v2 之前跑的，没有任何证据暴露它。所以变量集要能被单测直接拿来渲染真实模板，
// 见 prompt_contract_test.go。

// chapterGenerateVars 构造 chapter_generate 模板变量。
//
// 变量取值一律来自组装结果（asm），保证"模型看到的"与"快照记下的"是同一份文本。
func chapterGenerateVars(a *ctxengine.Assembly, scene string, targetWords int) map[string]any {
	return map[string]any{
		"TargetWords":       targetWords,
		"ChapterGoal":       a.Sections.ChapterGoal,
		"Scene":             scene,
		"CharacterContext":  a.Sections.Characters,
		"WorldContext":      a.Sections.World,
		"TimelineContext":   a.Sections.Timeline,
		"PreviousContext":   a.Sections.PrevSummary,
		"RetrievedOriginal": a.Sections.RetrievedOriginal,
		"RetrievedCreative": a.Sections.RetrievedCreative,
		"Instruction":       a.Sections.AuthorInstruction,
	}
}

// rewriteVars 构造 rewrite 模板变量（编辑器内 AI 操作：改写/扩写/续写…）。
func rewriteVars(a *ctxengine.Assembly, action, text string) map[string]any {
	return map[string]any{
		"Action":            action,
		"Text":              text,
		"Instruction":       a.Sections.AuthorInstruction,
		"CharacterContext":  a.Sections.Characters,
		"WorldContext":      a.Sections.World,
		"RetrievedCreative": a.Sections.RetrievedCreative,
	}
}

// consistencyVars 构造 consistency_check 模板变量。
//
// 五类上下文里"人物 / 世界 / 时间线 / 剧情"正好落在 §9.2 的预算段上；
// "原著继承映射"不是上下文预算的一部分（它是检查维度本身），原样单独传。
func consistencyVars(a *ctxengine.Assembly, inheritance, chapterText string) map[string]any {
	return map[string]any{
		"CharacterContext":   a.Sections.Characters,
		"WorldContext":       a.Sections.World,
		"TimelineContext":    a.Sections.Timeline,
		"PlotContext":        a.Sections.PrevSummary,
		"InheritanceContext": inheritance,
		"RetrievedOriginal":  a.Sections.RetrievedOriginal,
		"RetrievedCreative":  a.Sections.RetrievedCreative,
		"ChapterText":        trimChars(chapterText, maxAnalysisChars),
	}
}

// analyzeVars 构造 text_analyze 模板变量。
//
// asm 为空表示"只分析了选中文本、没有绑定章节"（没走上下文组装），
// 此时设定与检索段都为空 —— 如实留空，不编造。
func analyzeVars(a *ctxengine.Assembly, cctx ConsistencyContext, focus, text string) map[string]any {
	vars := map[string]any{
		"Focus":             focus,
		"Text":              trimChars(text, maxAnalysisChars),
		"CharacterContext":  cctx.Characters,
		"WorldContext":      cctx.World,
		"RetrievedCreative": "",
	}
	if a == nil {
		return vars
	}
	vars["Focus"] = a.Sections.AuthorInstruction
	vars["CharacterContext"] = a.Sections.Characters
	vars["WorldContext"] = a.Sections.World
	vars["RetrievedCreative"] = a.Sections.RetrievedCreative
	return vars
}
