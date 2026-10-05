// Package retrieval 是检索子系统（Phase 9 §9.1）：切块、索引、混合检索。
//
// 当前状态：**BM25-only**。任务书要求用 pgvector 做向量路，但本机 PostgreSQL 15.19
// 没有 vector 扩展、apt 也没有对应包、sudo 需密码 → 按任务书兜底走"Go 内 BM25，零依赖"，
// vector 记为阻塞项（见迁移 0019 注释与 docs/审查响应-*）。
package retrieval

import (
	"strings"

	"github.com/yanaoyi/novamindv2/backend/internal/domain"
)

const (
	// MaxChunkRunes 单块上限（任务书：≤800 汉字 ≈1200 tokens）
	MaxChunkRunes = 800
	// ChunkOverlapRunes 相邻块重叠（任务书：150 汉字）
	ChunkOverlapRunes = 150
)

// EstimateTokens 粗估 token 数：中文按 1 字 ≈0.67 token（任务书给定 len([]rune)/1.5）。
// 集中在这里实现，将来换 tiktoken 只需改这一处。
func EstimateTokens(text string) int {
	return int(float64(len([]rune(text))) / 1.5)
}

// ChunkChapter 把章节正文切成若干块。
//
// 策略：按字符窗口滑动，**尽量在换行处收口**（不把句子切两半），窗口之间保留 150 字重叠，
// 保证上下文不断档。空内容返回空；短于一块的内容整段返回（不硬凑）。
func ChunkChapter(content string) []domain.RetrievalChunk {
	text := normalizeWhitespace(content)
	if text == "" {
		return nil
	}
	runes := []rune(text)
	if len(runes) <= MaxChunkRunes {
		return []domain.RetrievalChunk{{Seq: 0, Content: text, TokenCount: EstimateTokens(text)}}
	}

	step := MaxChunkRunes - ChunkOverlapRunes
	var out []domain.RetrievalChunk
	for start := 0; start < len(runes); start += step {
		end := start + MaxChunkRunes
		if end > len(runes) {
			end = len(runes)
		}
		// 还在文本中间时，尝试在窗口后 25% 范围内最后一个换行处收口
		if end < len(runes) {
			lower := start + MaxChunkRunes*3/4
			for i := end - 1; i >= lower; i-- {
				if runes[i] == '\n' {
					end = i
					break
				}
			}
		}
		piece := strings.TrimSpace(string(runes[start:end]))
		if piece != "" {
			out = append(out, domain.RetrievalChunk{
				Seq: len(out), Content: piece, TokenCount: EstimateTokens(piece),
			})
		}
		if end >= len(runes) {
			break
		}
	}
	return out
}

// ChunkSingle 把"单条即一块"的内容（大纲节点、世界规则、人物、事件、记忆事实…）包成一块。
func ChunkSingle(content string) []domain.RetrievalChunk {
	text := normalizeWhitespace(content)
	if text == "" {
		return nil
	}
	// 极长的单条（例如超长世界规则）仍按章节规则切开，避免单块撑爆预算
	if len([]rune(text)) > MaxChunkRunes {
		return ChunkChapter(text)
	}
	return []domain.RetrievalChunk{{Seq: 0, Content: text, TokenCount: EstimateTokens(text)}}
}

// normalizeWhitespace 去掉首尾空白、把 3 个以上连续换行压成 2 个（段落边界保留），
// 行尾空白一并清掉——切块前统一，检索与展示都更稳定。
func normalizeWhitespace(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}
	out := strings.Join(lines, "\n")
	for strings.Contains(out, "\n\n\n") {
		out = strings.ReplaceAll(out, "\n\n\n", "\n\n")
	}
	return strings.TrimSpace(out)
}
