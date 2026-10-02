package task

import (
	"context"
	"errors"

	"github.com/yanaoyi/novamindv2/backend/internal/domain"
	"github.com/yanaoyi/novamindv2/backend/internal/service"
)

// RegisterAnalysisHandlers 注册各分析阶段任务。
//
// 注意：这些 handler **只产出提案**，不会写原著正式表 —— 作者在界面上通过后才落库（SPEC.md §22.2）。
func RegisterAnalysisHandlers(reg *Registry, analysis *service.AnalysisService, runner service.PromptRunner) {
	stages := []domain.AnalysisStage{
		domain.StageChapterSummary,
		domain.StageCharacterExtract,
		domain.StageWorldExtract,
		domain.StagePlotExtract,
	}
	for _, stage := range stages {
		st := stage
		reg.Register(st.TaskType(), func(ctx context.Context, t domain.Task, r Reporter) (map[string]any, error) {
			workID := InputString(t.Input, "work_id")
			if workID == "" {
				return nil, errors.New("任务缺少 work_id")
			}
			taskID := t.ID
			result, err := analysis.RunStage(ctx, workID, &taskID, st, runner, func(message string, percent int) {
				if r.Cancelled() {
					return
				}
				r.Progress(percent, message)
			})
			if err != nil {
				return nil, err
			}
			return map[string]any{
				"stage":             string(st),
				"proposals_created": result.CreatedCount,
			}, nil
		})
	}
}
