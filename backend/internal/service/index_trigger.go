package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/yanaoyi/novamindv2/backend/internal/domain"
)

// IndexTrigger 是"内容变了就重建这部分索引"的能力（由 TaskService 实现）。
//
// 为什么要有它（Phase 9 §9.1.4 的触发点）：索引此前只能靠人工调接口建立，
// 真实使用中 chunks 表一直是空的 —— 上下文里的检索段永远是空串，
// "越写越懂"实际上从来没有生效过。有了触发点，正文一落库索引就自己追上来。
//
// 用窄接口 + 可选注入：索引是增强能力，没接线时写作与导入照常（只是不留索引）。
type IndexTrigger interface {
	EnqueueIndex(ctx context.Context, workKind, workID, refKind, refID string) error
}

// indexDedupWindow 是同一 (作品, 来源) 的入队去重窗口。
//
// 为什么必须去重：编辑器是 1.5 秒自动保存，作者正常打字会在几十秒内触发几十次
// "重建本章索引"。单章重建本身很便宜（ReplaceChunks 先删后建），但任务表会被灌满、
// worker 会被这些任务占住。这里用进程内时间窗折叠突发。
//
// 为什么不做数据库级去重：那要给 tasks 加唯一索引/新迁移，收益只是"重启后前几秒
// 可能多入一次队"，不成比例。被折叠的那次不会丢数据：正文已经在库里，
// 后续任何一次保存（或全量重建）都会把索引追平。
const indexDedupWindow = 5 * time.Second

// taskDedupWindow 是"要调模型的任务"（事实抽取 / 自动一致性检查）的折叠窗口。
//
// 比索引入队窗口长：这两个任务每次都要真金白银地调模型，不能让作者打字把它们打爆。
// 折叠掉的那次同样不丢数据：任务执行时读到的是"当时的正文"，
// 之后再改再存会重新触发；记忆晚十几秒入库不影响正确性。
const taskDedupWindow = 20 * time.Second

// indexDedup 是入队去重器（独立成类型是为了能脱离仓储单测）。
type indexDedup struct {
	mu     sync.Mutex
	window time.Duration
	seen   map[string]time.Time
}

func newIndexDedup(window time.Duration) *indexDedup {
	return &indexDedup{window: window, seen: map[string]time.Time{}}
}

// allow 判断这个 key 现在能不能入队；顺便清理过期记录，避免 map 无限增长。
func (d *indexDedup) allow(key string, now time.Time) bool {
	if d == nil {
		return true
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	for k, at := range d.seen {
		if now.Sub(at) >= d.window {
			delete(d.seen, k)
		}
	}
	if at, ok := d.seen[key]; ok && now.Sub(at) < d.window {
		return false
	}
	d.seen[key] = now
	return true
}

// EnqueueIndex 按需入队一次索引重建（refKind/refID 为空表示全量重建）。
//
// 作品归属必须分别放进 work_id（原著）或 creative_work_id（二创）：
// tasks.work_id 的外键指向 original_works，把二创 id 塞进去会被外键拒绝。
func (s *TaskService) EnqueueIndex(ctx context.Context, workKind, workID, refKind, refID string) error {
	workKind = strings.TrimSpace(workKind)
	workID = strings.TrimSpace(workID)
	if workKind == "" || workID == "" {
		return fmt.Errorf("%w：索引入队缺少 work_kind 或 work_id", ErrBadRequest)
	}
	if workKind != domain.WorkKindOriginal && workKind != domain.WorkKindCreative {
		return fmt.Errorf("%w：work_kind 只能是 original 或 creative", ErrBadRequest)
	}
	if s.indexDedup != nil && !s.indexDedup.allow(
		workKind+"|"+workID+"|"+refKind+"|"+refID, time.Now()) {
		return nil
	}
	input := map[string]any{"work_kind": workKind, "work_id": workID}
	if strings.TrimSpace(refKind) != "" && strings.TrimSpace(refID) != "" {
		input["ref_kind"] = refKind
		input["ref_id"] = refID
	}
	in := EnqueueInput{Type: "index_chunks", Input: input}
	if workKind == domain.WorkKindOriginal {
		in.WorkID = &workID
	} else {
		in.CreativeWorkID = &workID
	}
	_, err := s.Enqueue(ctx, in)
	return err
}

// EnqueueFactExtract 排一次"从本章正文抽取记忆事实"的任务（Phase 9 §9.3.2）。
func (s *TaskService) EnqueueFactExtract(ctx context.Context, workID, chapterID string) error {
	if strings.TrimSpace(workID) == "" || strings.TrimSpace(chapterID) == "" {
		return fmt.Errorf("%w：事实抽取缺 work_id 或 chapter_id", ErrBadRequest)
	}
	if s.taskDedup != nil && !s.taskDedup.allow("facts|"+chapterID, time.Now()) {
		return nil
	}
	_, err := s.Enqueue(ctx, EnqueueInput{
		Type:           "extract_facts",
		CreativeWorkID: &workID,
		Input:          map[string]any{"chapter_id": chapterID},
	})
	return err
}

// EnqueueConsistencyCheck 排一次一致性检查（§9.4：记忆回写完成后自动查一次）。
func (s *TaskService) EnqueueConsistencyCheck(ctx context.Context, workID string, chapterIDs []string) error {
	if strings.TrimSpace(workID) == "" {
		return fmt.Errorf("%w：一致性检查缺 work_id", ErrBadRequest)
	}
	ids := chapterIDs
	if ids == nil {
		ids = []string{}
	}
	if s.taskDedup != nil && !s.taskDedup.allow("consistency|"+workID+"|"+strings.Join(ids, ","), time.Now()) {
		return nil
	}
	_, err := s.Enqueue(ctx, EnqueueInput{
		Type:           "consistency_check",
		CreativeWorkID: &workID,
		Input:          map[string]any{"work_id": workID, "chapter_ids": ids},
	})
	return err
}
