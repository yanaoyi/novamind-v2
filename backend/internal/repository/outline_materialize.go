package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/yanaoyi/novamindv2/backend/internal/domain"
)

// MaterializeVolume 是落成时要新建的卷。
type MaterializeVolume struct {
	Title    string
	Summary  string
	Sequence int
}

// MaterializeChapter 是落成时要新建的章节。
// VolumeIndex 指向 ExistingVolumeIDs 或新建卷列表的下标；-1 表示不挂卷。
type MaterializeChapter struct {
	VolumeIndex   int
	ChapterNo     int
	Title         string
	Summary       string
	Purpose       string
	Conflict      string
	Outcome       string
	OutlineNodeID string
}

// MaterializePlan 是「大纲落成」的完整写入计划（由 service 计算，repo 只负责一次事务写完）。
type MaterializePlan struct {
	ExistingVolumeIDs []string
	NewVolumes        []MaterializeVolume
	Chapters          []MaterializeChapter
}

// MaterializeOutcome 是落成结果。
type MaterializeOutcome struct {
	VolumeIDs  []string
	ChapterIDs []string
}

// MaterializeOutline 在一次事务里完成"新建卷 + 新建章节"。
//
// 为什么要收成一个仓储方法：落成要写 creative_volumes 与 creative_chapters 两张表，
// 用 service 循环调用各自的仓储方法会留下"卷建好了、章节写一半失败"的半成品
// （代码审查 P1-2）。放这里就能整体回滚，事务边界清清楚楚。
func (r *WritingRepo) MaterializeOutline(
	ctx context.Context,
	workID string,
	plan MaterializePlan,
) (*MaterializeOutcome, error) {
	now := time.Now().UTC()
	out := &MaterializeOutcome{VolumeIDs: make([]string, 0, len(plan.NewVolumes)), ChapterIDs: make([]string, 0, len(plan.Chapters))}

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1) 新建卷，并把"卷下标 → 实际 ID"记下来
		newVolumeIDs := make([]string, 0, len(plan.NewVolumes))
		for _, v := range plan.NewVolumes {
			id, err := uuid.NewV7()
			if err != nil {
				return fmt.Errorf("生成卷 ID 失败: %w", err)
			}
			model := volumeModel{
				ID: id.String(), CreativeWorkID: workID, Title: v.Title, Summary: v.Summary,
				Sequence: v.Sequence, CreatedAt: now, UpdatedAt: now,
			}
			if err := tx.Create(&model).Error; err != nil {
				if isForeignKeyViolation(err) {
					return domain.ErrCreativeNotFound
				}
				return fmt.Errorf("创建卷失败: %w", err)
			}
			newVolumeIDs = append(newVolumeIDs, model.ID)
			out.VolumeIDs = append(out.VolumeIDs, model.ID)
		}
		volumeIDAt := func(index int) *string {
			if index < 0 {
				return nil
			}
			if index < len(plan.ExistingVolumeIDs) {
				id := plan.ExistingVolumeIDs[index]
				return &id
			}
			offset := index - len(plan.ExistingVolumeIDs)
			if offset < 0 || offset >= len(newVolumeIDs) {
				return nil
			}
			id := newVolumeIDs[offset]
			return &id
		}

		// 2) 写章节
		for _, c := range plan.Chapters {
			id, err := uuid.NewV7()
			if err != nil {
				return fmt.Errorf("生成章节 ID 失败: %w", err)
			}
			chapter := &domain.CreativeChapter{
				CreativeWorkID: workID, VolumeID: volumeIDAt(c.VolumeIndex), ChapterNo: c.ChapterNo,
				Title: c.Title, Summary: c.Summary, Purpose: c.Purpose, Conflict: c.Conflict,
				Outcome: c.Outcome, Status: domain.ChapterDraft,
			}
			chapter.Normalize()
			if err := chapter.Validate(); err != nil {
				return err
			}
			var nodeID *string
			if c.OutlineNodeID != "" {
				node := c.OutlineNodeID
				nodeID = &node
			}
			model := chapterModel{
				ID: id.String(), CreativeWorkID: workID, VolumeID: chapter.VolumeID,
				OutlineNodeID: nodeID, ChapterNo: c.ChapterNo, Title: c.Title, Summary: c.Summary,
				Content: "", Status: string(domain.ChapterDraft), WordCount: 0,
				Purpose: c.Purpose, Conflict: c.Conflict, Outcome: c.Outcome,
				CreatedAt: now, UpdatedAt: now,
			}
			if err := tx.Create(&model).Error; err != nil {
				if isUniqueViolation(err) {
					return fmt.Errorf("该章节号已存在（章号 %d）", c.ChapterNo)
				}
				if isForeignKeyViolation(err) {
					return domain.ErrCreativeNotFound
				}
				return fmt.Errorf("创建章节失败: %w", err)
			}
			out.ChapterIDs = append(out.ChapterIDs, model.ID)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}
