package service

import (
	"strings"
	"testing"

	"github.com/yanaoyi/novamindv2/backend/internal/ai"
	ctxengine "github.com/yanaoyi/novamindv2/backend/internal/context"
)

// TestPromptTemplatesRenderWithServiceVars 是"模板 ↔ 调用方变量"的契约测试。
//
// 引擎是 missingkey=error：模板引用了变量而调用方没传 → 渲染直接失败。
// 2026-10-05 已经踩过一次（chapter_generate.v2 先进仓库、调用方还没补变量，
// 当时 e2e 是在引入 v2 之前跑的，没有任何证据暴露它）。所以这里不测"变量名对得上"，
// 而是直接拿服务真正会传的变量集去渲染真正会被选中的那版模板。
func TestPromptTemplatesRenderWithServiceVars(t *testing.T) {
	engine, err := ai.NewEngine()
	if err != nil {
		t.Fatalf("Prompt 引擎构建失败: %v", err)
	}

	asm := ctxengine.AssembleForChapter(
		ctxengine.ChapterFacts{
			ChapterGoal:      "让沈砚发现账册异常",
			CharacterContext: "- 沈砚（INHERITED）：克制",
			WorldContext:     "世界：江城",
			TimelineContext:  "- [NEW] 灯会当夜：库房起火",
			PreviousContext:  "第2章 对峙：与督军的人结怨",
		},
		[]ctxengine.RetrievalHit{{ChunkID: "c1", RefKind: "chapter", Seq: 1, Content: "第一章埋下青铜钥匙。", Score: 0.91}},
		[]ctxengine.RetrievalHit{{ChunkID: "c2", RefKind: "memory_fact", Content: "沈砚左臂受伤，无法用剑。", Score: 0.77}},
		"控制在 800 字以内",
	)

	cases := []struct {
		prompt    string
		version   string
		vars      map[string]any
		wantInOut string
	}{
		{"chapter_generate", "v3", chapterGenerateVars(asm, "夜审账册", 1500), "青铜钥匙"},
		{"rewrite", "v3", rewriteVars(asm, string(RewriteActionExpand), "他推开门。"), "左臂受伤"},
		{"consistency_check", "v3", consistencyVars(asm, "原著人物A → 二创人物B", "他推开门，右脚一软。"), "左臂受伤"},
		{"text_analyze", "v2", analyzeVars(asm, ConsistencyContext{}, "关注人物状态", "他推开门。"), "左臂受伤"},
	}

	for _, c := range cases {
		prompt, err := engine.Get(c.prompt, "")
		if err != nil {
			t.Fatalf("取模板 %s 失败: %v", c.prompt, err)
		}
		if prompt.Version != c.version {
			t.Errorf("%s 生效版本应为 %s（Phase 9 接线版），实际 %s", c.prompt, c.version, prompt.Version)
		}
		out, err := prompt.Render(c.vars)
		if err != nil {
			t.Errorf("模板 %s.%s 用服务的变量集渲染失败: %v", c.prompt, prompt.Version, err)
			continue
		}
		if !strings.Contains(out, c.wantInOut) {
			t.Errorf("模板 %s 的渲染结果里没带上检索内容 %q：\n%s", c.prompt, c.wantInOut, out)
		}
	}
}

// 渲染结果必须能反映"作者指令"，否则 §9.2 的"作者指令最后才被截断"就无从谈起。
func TestChapterGenerateVarsCarryTruncatedInstruction(t *testing.T) {
	asm := ctxengine.AssembleForChapter(
		ctxengine.ChapterFacts{ChapterGoal: "目标"},
		nil, nil, "不要写成大纲体",
	)
	vars := chapterGenerateVars(asm, "场景", 1000)
	if vars["Instruction"] != "不要写成大纲体" {
		t.Errorf("作者指令应原样进模板变量，实际 %v", vars["Instruction"])
	}
	if vars["TargetWords"] != 1000 {
		t.Errorf("目标字数应进模板变量，实际 %v", vars["TargetWords"])
	}
}
