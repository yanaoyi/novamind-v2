package service

import (
	"strings"
	"testing"
)

// 结构化输出与正文输出必须用两套系统提示：
// 正文类提示词若被套上「只输出 JSON」+ response_format=json_object，
// 上游会返回空内容（2026-10-04 端到端验收抓到的真缺陷，表现为「AI 写本章」随机失败）。
func TestSystemPromptSeparatesJSONAndProse(t *testing.T) {
	jsonPrompt := systemPrompt(true)
	if !strings.Contains(jsonPrompt, "JSON") {
		t.Errorf("结构化模式应要求 JSON，实际: %s", jsonPrompt)
	}

	prose := systemPrompt(false)
	if strings.Contains(prose, "JSON") {
		t.Errorf("正文模式不应要求 JSON，实际: %s", prose)
	}
	if !strings.Contains(prose, "直接输出") {
		t.Errorf("正文模式应要求直接输出内容，实际: %s", prose)
	}
	if jsonPrompt == prose {
		t.Error("两种模式的系统提示不能相同")
	}
}
