package service

import (
	"errors"
	"testing"
)

func TestExtractJSONObjectPlain(t *testing.T) {
	obj, err := ExtractJSONObject(`{"summary":"本章讲了一个重逢","key_points":["a","b"]}`)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if obj["summary"] != "本章讲了一个重逢" {
		t.Errorf("内容不正确: %v", obj)
	}
}

func TestExtractJSONObjectFromCodeFence(t *testing.T) {
	// 模型即使被要求"只输出 JSON"，也常常包上代码块
	content := "```json\n{\"characters\":[{\"name\":\"林默\"}]}\n```"
	obj, err := ExtractJSONObject(content)
	if err != nil {
		t.Fatalf("应能从代码块里抽出 JSON: %v", err)
	}
	chars, ok := obj["characters"].([]any)
	if !ok || len(chars) != 1 {
		t.Fatalf("解析结果不正确: %v", obj)
	}
}

func TestExtractJSONObjectWithProse(t *testing.T) {
	content := "好的，这是分析结果：\n{\"world\":{\"name\":\"江城\"}}\n希望对你有帮助。"
	obj, err := ExtractJSONObject(content)
	if err != nil {
		t.Fatalf("应能从解释文字中抽出 JSON: %v", err)
	}
	world, ok := obj["world"].(map[string]any)
	if !ok || world["name"] != "江城" {
		t.Fatalf("解析结果不正确: %v", obj)
	}
}

func TestExtractJSONObjectInvalid(t *testing.T) {
	cases := []string{
		"",
		"模型今天不想干活",
		"{这不是合法 JSON}",
		"```json\n[1,2,3]\n```", // 只有数组，没有对象
	}
	for _, content := range cases {
		if _, err := ExtractJSONObject(content); !errors.Is(err, ErrModelJSONInvalid) {
			t.Errorf("内容 %q 应返回 ErrModelJSONInvalid，实际 %v", content, err)
		}
	}
}

func TestExtractJSONArray(t *testing.T) {
	arr, err := ExtractJSONArray("```json\n[{\"name\":\"甲\"},{\"name\":\"乙\"}]\n```")
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if len(arr) != 2 {
		t.Fatalf("应解析出 2 项，实际 %d", len(arr))
	}
	if _, err := ExtractJSONArray("没有数组"); !errors.Is(err, ErrModelJSONInvalid) {
		t.Errorf("无数组时应报错，实际 %v", err)
	}
}

func TestParseDNAFromModelPayload(t *testing.T) {
	payload := map[string]any{
		"personality": map[string]any{"text": "克制", "weight": float64(90)},
		"values":      map[string]any{"text": "重承诺", "weight": float64(80)},
	}
	dna := parseDNA(payload)
	if dna.Personality.Weight != 90 || dna.Personality.Text != "克制" {
		t.Errorf("人格维度解析不正确: %+v", dna.Personality)
	}
	if dna.Values.Weight != 80 {
		t.Errorf("价值观维度解析不正确: %+v", dna.Values)
	}
	if dna.Ability.Weight != 0 {
		t.Errorf("未提供的维度应为零值，实际 %+v", dna.Ability)
	}

	// 非法结构不应 panic，返回零值
	zero := parseDNA("不是对象")
	if zero.Personality.Weight != 0 || zero.Values.Text != "" {
		t.Errorf("非法入参应返回零值，实际 %+v", zero)
	}
}

func TestStrAndIntHelpers(t *testing.T) {
	m := map[string]any{
		"name":       "  林默  ",
		"importance": float64(5),
		"count":      "12",
		"list":       []any{"甲", " 乙 ", "", 3},
	}
	if got := strVal(m, "name"); got != "林默" {
		t.Errorf("字符串应去空白，实际 %q", got)
	}
	if got := intVal(m, "importance"); got != 5 {
		t.Errorf("浮点转整数失败，实际 %d", got)
	}
	if got := intVal(m, "count"); got != 12 {
		t.Errorf("字符串数字应能解析，实际 %d", got)
	}
	if got := intVal(m, "missing"); got != 0 {
		t.Errorf("缺失键应为 0，实际 %d", got)
	}
	names := strSlice(m["list"])
	if len(names) != 2 || names[0] != "甲" || names[1] != "乙" {
		t.Errorf("字符串切片应过滤空值并去空白，实际 %v", names)
	}
}
