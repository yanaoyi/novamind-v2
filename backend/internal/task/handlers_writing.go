package task

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/yanaoyi/novamindv2/backend/internal/domain"
	"github.com/yanaoyi/novamindv2/backend/internal/service"
)

// 写作与一致性相关的任务类型。
const (
	TypeWritingChapter   = "writing_chapter"
	TypeConsistencyCheck = "consistency_check"
)

// RegisterWritingHandlers 注册「AI 写本章」与「一致性检查」任务。
func RegisterWritingHandlers(reg *Registry, writing *service.WritingService, runner service.PromptRunner) {
	reg.Register(TypeWritingChapter, func(ctx context.Context, t domain.Task, r Reporter) (map[string]any, error) {
		chapterID := InputString(t.Input, "chapter_id")
		if chapterID == "" {
			return nil, errors.New("任务缺少 chapter_id")
		}
		targetWords := 2000
		if v, ok := t.Input["target_words"].(float64); ok && v > 0 {
			targetWords = int(v)
		}
		instruction := InputString(t.Input, "instruction")

		draft, err := writing.GenerateChapterDraft(ctx, chapterID, runner, targetWords, instruction, func(stage string, percent int) {
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
		// 写入正文（会自动生成一个版本）
		updated, _, err := writing.UpdateChapter(ctx, chapterID, service.UpdateChapterInput{Content: &draft})
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"chapter_id": chapterID,
			"word_count": updated.WordCount,
			"summary":    fmt.Sprintf("已生成 %d 字", updated.WordCount),
		}, nil
	})

	reg.Register(TypeConsistencyCheck, func(ctx context.Context, t domain.Task, r Reporter) (map[string]any, error) {
		workID := InputString(t.Input, "work_id")
		if workID == "" {
			return nil, errors.New("任务缺少 work_id")
		}
		chapterIDs := []string{}
		if raw, ok := t.Input["chapter_ids"].([]any); ok {
			for _, item := range raw {
				if s, ok := item.(string); ok && s != "" {
					chapterIDs = append(chapterIDs, s)
				}
			}
		}
		result, err := writing.CheckConsistency(ctx, workID, chapterIDs, runner, func(stage string, percent int) {
			if r.Cancelled() {
				return
			}
			r.Progress(percent, stage)
		})
		if err != nil {
			return nil, err
		}
		summary := fmt.Sprintf("检查 %d 章，发现 %d 个问题", result.Checked, result.Created)
		if len(result.FailedChapters) > 0 {
			summary += fmt.Sprintf("；%d 章检查失败（模型输出不是合法 JSON）：%s",
				len(result.FailedChapters), strings.Join(result.FailedChapters, "、"))
		}
		return map[string]any{
			"checked":         result.Checked,
			"issues_created":  result.Created,
			"failed_chapters": result.FailedChapters,
			"summary":         summary,
		}, nil
	})
}
