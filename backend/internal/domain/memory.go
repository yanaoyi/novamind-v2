package domain

import (
	"errors"
	"strings"
	"time"
)

// 记忆事实的类型（与迁移 0023 的 CHECK 约束一致）。
type FactKind string

const (
	FactCharacterState FactKind = "character_state"
	FactWorldState     FactKind = "world_state"
	FactEvent          FactKind = "event"
	FactItem           FactKind = "item"
	FactRelationship   FactKind = "relationship"
	FactPlot           FactKind = "plot"
)

// Valid 判断事实类型是否合法。
func (k FactKind) Valid() bool {
	switch k {
	case FactCharacterState, FactWorldState, FactEvent, FactItem, FactRelationship, FactPlot:
		return true
	default:
		return false
	}
}

// FactKinds 返回全部合法类型（前端/文档/校验共用一份）。
func FactKinds() []string {
	return []string{
		string(FactCharacterState), string(FactWorldState), string(FactEvent),
		string(FactItem), string(FactRelationship), string(FactPlot),
	}
}

// ErrFactSubjectEmpty / ErrFactTextEmpty 表示记忆事实的关键字段为空。
var (
	ErrFactSubjectEmpty = errors.New("记忆事实缺少主体")
	ErrFactTextEmpty    = errors.New("记忆事实内容为空")
	ErrFactKindInvalid  = errors.New("记忆事实类型不合法")
	// ErrChapterSummaryNotFound 表示该章还没有摘要。
	ErrChapterSummaryNotFound = errors.New("章节摘要不存在")
)

// MemoryFact 是一条长篇记忆事实（Phase 9 §9.3）。
//
// SupersededBy 非空表示"这条已被新事实替代"——旧行保留，用于追溯演变过程。
type MemoryFact struct {
	ID             string
	OwnerUserID    string
	CreativeWorkID string
	Kind           FactKind
	Subject        string
	Fact           string
	ChapterID      *string
	SupersededBy   *string
	CreatedAt      time.Time
}

// Normalize 清洗输入（去空白，避免" 沈砚 "与"沈砚"被当成两个主体）。
func (f *MemoryFact) Normalize() {
	f.Subject = strings.TrimSpace(f.Subject)
	f.Fact = strings.TrimSpace(f.Fact)
}

// Validate 校验一条事实；AI 抽取的原始输出也要过这一关才能入库。
func (f *MemoryFact) Validate() error {
	if !f.Kind.Valid() {
		return ErrFactKindInvalid
	}
	if strings.TrimSpace(f.Subject) == "" {
		return ErrFactSubjectEmpty
	}
	if strings.TrimSpace(f.Fact) == "" {
		return ErrFactTextEmpty
	}
	return nil
}

// ChapterSummary 是一章的摘要（Episodic 层）。
type ChapterSummary struct {
	ChapterID   string
	OwnerUserID string
	Summary     string
	CreatedAt   time.Time
}
