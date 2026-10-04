package ai

import (
	"errors"
	"strings"
	"testing"
)

func TestPromptEngineLoadsTemplates(t *testing.T) {
	engine, err := NewEngine()
	if err != nil {
		t.Fatalf("加载模板失败: %v", err)
	}
	metas := engine.List()
	if len(metas) < 7 {
		t.Fatalf("模板数量偏少（应覆盖 original/character/world/plot/outline/writing/review），实际 %d", len(metas))
	}

	names := map[string]bool{}
	for _, m := range metas {
		names[m.Name] = true
		if m.Version == "" {
			t.Errorf("模板 %s 缺少版本", m.Name)
		}
	}
	for _, want := range []string{
		"chapter_summary", "character_extract", "world_extract",
		"plot_extract", "outline_generate", "chapter_generate", "consistency_check",
	} {
		if !names[want] {
			t.Errorf("缺少模板 %s", want)
		}
	}
}

func TestPromptEngineGetAndRender(t *testing.T) {
	engine, err := NewEngine()
	if err != nil {
		t.Fatalf("加载模板失败: %v", err)
	}

	prompt, err := engine.Get("chapter_summary", "")
	if err != nil {
		t.Fatalf("取模板失败: %v", err)
	}
	if prompt.Version != "v1" {
		t.Errorf("空版本应取最新，实际 %s", prompt.Version)
	}

	rendered, err := prompt.Render(map[string]any{
		"ChapterTitle": "第一章 初遇",
		"ChapterText":  "林默站在月台上。",
	})
	if err != nil {
		t.Fatalf("渲染失败: %v", err)
	}
	if !strings.Contains(rendered, "第一章 初遇") || !strings.Contains(rendered, "林默站在月台上。") {
		t.Errorf("渲染结果缺少变量内容: %s", rendered)
	}
	if !strings.Contains(rendered, "\"summary\"") {
		t.Error("渲染结果里应包含要求的 JSON 结构说明")
	}
}

func TestPromptEngineMissingTemplate(t *testing.T) {
	engine, err := NewEngine()
	if err != nil {
		t.Fatalf("加载模板失败: %v", err)
	}
	if _, err := engine.Get("no_such_prompt", ""); !errors.Is(err, ErrPromptNotFound) {
		t.Fatalf("应返回 ErrPromptNotFound，实际 %v", err)
	}
	if _, err := engine.Get("chapter_summary", "v99"); !errors.Is(err, ErrPromptNotFound) {
		t.Fatalf("不存在的版本应返回 ErrPromptNotFound，实际 %v", err)
	}
}

// TestOutlineGenerateRendersWithServiceVars 守住一条真实踩过的坑：
// 服务层给 outline_generate 传的变量名必须和模板里用的名字一致，
// 否则渲染直接失败（missingkey=error），接口报 500。
func TestOutlineGenerateRendersWithServiceVars(t *testing.T) {
	engine, err := NewEngine()
	if err != nil {
		t.Fatalf("加载模板失败: %v", err)
	}
	prompt, err := engine.Get("outline_generate", "")
	if err != nil {
		t.Fatalf("取模板失败: %v", err)
	}

	_, err = prompt.Render(map[string]any{
		"WorkTitle":        "测试作品",
		"Requirement":      "三卷结构",
		"CharacterContext": "主角：沈砚",
		"WorldContext":     "北境军镇",
		"PlotContext":      "查账线",
	})
	if err != nil {
		t.Fatalf("outline_generate 渲染失败（服务层变量名与模板不一致）: %v", err)
	}
}

func TestPromptRenderFailsOnMissingKey(t *testing.T) {
	// 模板里用到的变量没传 → 必须报错（fail fast），而不是渲染出空值
	engine, err := NewEngine()
	if err != nil {
		t.Fatalf("加载模板失败: %v", err)
	}
	prompt, err := engine.Get("chapter_summary", "v1")
	if err != nil {
		t.Fatalf("取模板失败: %v", err)
	}
	if _, err := prompt.Render(map[string]any{"ChapterTitle": "只有标题"}); err == nil {
		t.Fatal("缺少模板变量时应渲染失败")
	}
}
