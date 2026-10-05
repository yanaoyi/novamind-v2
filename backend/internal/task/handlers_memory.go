package task

import (
	"context"
	"errors"

	"github.com/yanaoyi/novamindv2/backend/internal/domain"
	"github.com/yanaoyi/novamindv2/backend/internal/service"
)

// TypeExtractFacts 是"从章节正文抽取记忆事实"的任务类型（Phase 9 §9.3.2）。
const TypeExtractFacts = "extract_facts"

// RegisterMemoryHandlers 注册长篇记忆相关任务。
//
// 抽取失败**不影响写作**：章节正文早已保存，任务失败只体现在任务中心，
// 作者重试或下一次保存都会重新触发。
func RegisterMemoryHandlers(reg *Registry, memory *service.MemoryService, runner service.PromptRunner) {
	reg.Register(TypeExtractFacts, func(ctx context.Context, t domain.Task, r Reporter) (map[string]any, error) {
		chapterID := InputString(t.Input, "chapter_id")
		if chapterID == "" {
			return nil, errors.New("任务缺少 chapter_id")
		}
		result, err := memory.ExtractFacts(ctx, chapterID, runner, func(stage string, percent int) {
			if r.Cancelled() {
				return
			}
			r.Progress(percent, stage)
		})
		if err != nil {
			return nil, err
		}
		if r.Cancelled() {
			return nil, errors.New("任务已取消")
		}
		return map[string]any{
			"chapter_id":       result.ChapterID,
			"facts_created":    result.FactsCreated,
			"facts_superseded": result.FactsSuperseded,
			"indexed":          result.Indexed,
			"summary_chars":    result.SummaryChars,
		}, nil
	})
}
