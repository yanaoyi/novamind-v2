package service

import (
	"context"
	"fmt"

	"github.com/yanaoyi/novamindv2/backend/internal/ai"
)

// jsonRetryHint 是"上一次输出没有形成合法 JSON"时追加给模型的收敛提示（规格书 §58：错误处理要能自动修复重试）。
//
// 真实场景（2026-10-04 端到端验收遇到）：让模型一次产出 3 卷 9 节 18 章的大纲，
// 输出被 max_tokens 截断、JSON 没有闭合 → unexpected end of JSON input。
// 重试时把"更简洁、必须闭合"讲清楚，命中率远高于原样重试。
const jsonRetryHint = "\n\n【上一次的输出没有形成合法 JSON，通常是被截断了】请这次更简洁，并确保 JSON 完整闭合：" +
	"卷不超过 3 个、每卷节不超过 3 个、每节章不超过 5 个；只输出一个完整的 JSON 对象，不要解释文字或代码块标记。"

// RunJSONPrompt 调用模型并解析 JSON；解析失败时带收敛提示重试一次，仍失败则返回带原因的错误。
//
// 统一放在这里的原因：所有"要结构化输出"的能力（大纲/人物/剧情/场景生成、就地分析）
// 都会遇到同一个失败模式——截断。重试策略只该有一份，避免各调用点各写各的。
func RunJSONPrompt(
	ctx context.Context,
	runner PromptRunner,
	promptName string,
	vars map[string]any,
	maxAttempts int,
) (map[string]any, error) {
	if maxAttempts <= 0 {
		maxAttempts = 2
	}
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		payload := vars
		if attempt > 1 {
			payload = withExtraInstruction(vars, jsonRetryHint)
		}
		reply, err := runner.RunPrompt(ctx, promptName, payload)
		if err != nil {
			return nil, err
		}
		obj, err := ExtractJSONObject(reply)
		if err == nil {
			return obj, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("模型输出不是合法 JSON（已重试 %d 次）：%w", maxAttempts, lastErr)
}

// RunJSONPromptValidated 在 RunJSONPrompt 之上加一道 JSON Schema 校验（Phase 9 §9.5）。
//
// 与"JSON 解析失败重试"的区别：解析失败是**格式**问题（通常被截断），
// 校验失败是**内容**问题（枚举越界、缺字段、长度越界）——后者必须把校验器给出的
// 具体问题喂回模型，它才知道该改哪里（§9.3.2 要求的"修一次"）。
func RunJSONPromptValidated(
	ctx context.Context,
	runner PromptRunner,
	promptName, schemaName string,
	vars map[string]any,
	maxAttempts int,
) (map[string]any, error) {
	if maxAttempts <= 0 {
		maxAttempts = 2
	}
	var lastErr error
	payload := vars
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		obj, err := RunJSONPrompt(ctx, runner, promptName, payload, 1)
		if err != nil {
			lastErr = err
		} else if err := ai.ValidateObject(obj, schemaName); err != nil {
			lastErr = err
			// 把校验器的问题清单原样喂回去 —— 它带 JSON 路径与允许值，模型能照着改
			payload = withExtraInstruction(payload,
				"\n\n【上一次的输出没有通过结构校验】请按下面的问题逐条修正后重新输出完整的 JSON："+
					"\n"+err.Error())
		} else {
			return obj, nil
		}
	}
	return nil, fmt.Errorf("模型输出两次都不合格（已重试 %d 次）：%w", maxAttempts, lastErr)
}

// withExtraInstruction 复制一份变量并给"作者要求"追加提示，不改动调用方的 map。
func withExtraInstruction(vars map[string]any, hint string) map[string]any {
	out := make(map[string]any, len(vars)+2)
	for k, v := range vars {
		out[k] = v
	}
	appended := false
	for _, key := range []string{"Instruction", "Requirement"} {
		existing, ok := out[key].(string)
		if !ok {
			continue
		}
		out[key] = existing + hint
		appended = true
	}
	if !appended {
		out["Instruction"] = hint
	}
	return out
}
