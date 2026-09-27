package task

import (
	"context"
	"errors"
	"fmt"

	"github.com/yanaoyi/novamindv2/backend/internal/domain"
	"github.com/yanaoyi/novamindv2/backend/internal/service"
)

// 任务类型常量。
const (
	// TypeOriginalReparse 用已保存的源文件重新解析原著章节。
	TypeOriginalReparse = "original_reparse"
)

// RegisterOriginalHandlers 注册原著相关任务。
func RegisterOriginalHandlers(reg *Registry, originals *service.OriginalService) {
	reg.Register(TypeOriginalReparse, func(ctx context.Context, t domain.Task, r Reporter) (map[string]any, error) {
		workID := InputString(t.Input, "work_id")
		if workID == "" {
			return nil, errors.New("任务缺少 work_id")
		}
		r.Progress(5, "准备重新解析")

		result, err := originals.Reparse(ctx, workID, func(stage string, percent int) {
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
			"chapter_count": result.ChapterCount,
			"char_count":    result.CharCount,
			"encoding":      result.Encoding,
			"file_name":     result.FileName,
			"summary":       fmt.Sprintf("重新解析完成：%d 章 / %d 字", result.ChapterCount, result.CharCount),
		}, nil
	})
}
