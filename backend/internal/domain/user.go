package domain

import (
	"errors"
	"time"
)

// 用户相关错误（Phase 9 最小骨架）。
var ErrUserNotFound = errors.New("用户不存在")

// UserRole 是用户角色：admin 可跨用户读取（检索的 ownerScope 豁免），user 只看自己的。
type UserRole string

const (
	UserRoleAdmin UserRole = "admin"
	UserRoleUser  UserRole = "user"
)

// Valid 判断角色是否合法。
func (r UserRole) Valid() bool { return r == UserRoleAdmin || r == UserRoleUser }

// User 是用户（Phase 9 起的最小骨架）。
//
// 说明：本期只落地"数据归属"所需的最小字段；登录/口令/会话属于多用户 Phase A/B，另行排期。
type User struct {
	ID          string
	Username    string
	DisplayName string
	Role        UserRole
	Status      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// IsAdmin 判断是否管理员。
func (u *User) IsAdmin() bool { return u.Role == UserRoleAdmin }
