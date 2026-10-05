// Package context 是写作时的上下文组装（Phase 9 §9.2 Context Engine）。
package context

import "github.com/yanaoyi/novamindv2/backend/internal/retrieval"

// EstimateTokens 估算 token 数。
//
// 复用 retrieval 里的实现，保证"切块预算"与"上下文预算"用的是同一把尺子
// （两边各写一份公式迟早会漂移）。将来换 tiktoken 只需改 retrieval 那一处。
func EstimateTokens(text string) int {
	return retrieval.EstimateTokens(text)
}

// runesForTokens 反推"多少个汉字约等于这么多 token"（用于按预算截断）。
func runesForTokens(tokens int) int {
	if tokens <= 0 {
		return 0
	}
	return int(float64(tokens) * 1.5)
}

// truncateToTokens 把文本截到不超过 tokens（按 rune 截，不切坏 UTF-8）。
func truncateToTokens(text string, tokens int) string {
	limit := runesForTokens(tokens)
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	if limit <= 0 {
		return ""
	}
	return string(runes[:limit])
}
