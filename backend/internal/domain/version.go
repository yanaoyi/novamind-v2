package domain

import (
	"errors"
	"time"
)

// 版本相关错误。
var (
	ErrVersionNotFound  = errors.New("版本不存在")
	ErrVersionTypeBad   = errors.New("版本实体类型非法")
	ErrVersionNoInvalid = errors.New("版本号必须为正整数")
)

// EntityVersionType 是可版本化的实体类型（规格书 §59）。
type EntityVersionType string

const (
	VersionCreativeCharacter EntityVersionType = "creative_character"
	VersionCreativeWorld     EntityVersionType = "creative_world"
	VersionCreativeOutline   EntityVersionType = "creative_outline"
	// VersionCreativeOutlineTree 是大纲树（§27 的 Outline + OutlineNode）自己的版本类型，
	// 与旧的 creative_outline（卷 + 章节大纲）区分：后者的 entity_id 是作品 id，前者是大纲 id。
	VersionCreativeOutlineTree EntityVersionType = "creative_outline_tree"
)

// Valid 判断类型是否合法。
func (t EntityVersionType) Valid() bool {
	switch t {
	case VersionCreativeCharacter, VersionCreativeWorld, VersionCreativeOutline, VersionCreativeOutlineTree:
		return true
	default:
		return false
	}
}

// EntityVersion 是一次状态快照。
//
// 语义要说清楚：它是「某一刻这个实体长什么样」，不是增量补丁。
// 因此恢复操作只能把快照里记录的字段写回去，不能让已经新建的实体凭空消失
// （删除类操作不做反向回滚，避免作者丢了东西还以为是回滚的锅）。
type EntityVersion struct {
	ID             string
	EntityType     EntityVersionType
	EntityID       string
	CreativeWorkID string
	VersionNo      int
	Payload        map[string]any
	Note           string
	CreatedAt      time.Time
}
