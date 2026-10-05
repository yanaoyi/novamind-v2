package domain

import (
	"errors"
	"time"
)

var (
	ErrSnapshotNotFound = errors.New("上下文快照不存在")
	ErrSnapshotKindBad  = errors.New("上下文快照类型非法")
)

// SnapshotKind 是触发快照的 AI 调用类型（与迁移 0020 的 CHECK 一致）。
type SnapshotKind string

const (
	SnapshotGenerate    SnapshotKind = "generate"
	SnapshotContinue    SnapshotKind = "continue"
	SnapshotRewrite     SnapshotKind = "rewrite"
	SnapshotExpand      SnapshotKind = "expand"
	SnapshotAnalyze     SnapshotKind = "analyze"
	SnapshotConsistency SnapshotKind = "consistency"
)

// Valid 判断类型是否合法。
func (k SnapshotKind) Valid() bool {
	switch k {
	case SnapshotGenerate, SnapshotContinue, SnapshotRewrite, SnapshotExpand, SnapshotAnalyze, SnapshotConsistency:
		return true
	default:
		return false
	}
}

// ContextSnapshot 是一次 AI 调用的上下文快照（只增不改）。
type ContextSnapshot struct {
	ID             string
	OwnerUserID    string
	CreativeWorkID string
	ChapterID      *string
	Kind           SnapshotKind
	// Snapshot 里放：8 段组装结果（含截断标记与 token 数）+ 检索命中（chunk id/score）
	// + 使用的模型 + prompt 模板版本 + 总 token。结构由 service 组装，DB 只当 JSONB 存。
	Snapshot  map[string]any
	CreatedAt time.Time
}
