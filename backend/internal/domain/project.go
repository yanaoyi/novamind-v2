// Package domain 存放领域实体与领域规则。
// 约束：本包不得依赖 gin / gorm / 任何外部框架，保证领域模型可被独立测试。
package domain

import (
	"errors"
	"strings"
	"time"
)

// ProjectType 区分原著工程与二创工程（SPEC.md §4）。
type ProjectType string

const (
	// ProjectTypeOriginal 原著工程：承载 Original Model。
	ProjectTypeOriginal ProjectType = "ORIGINAL"
	// ProjectTypeCreative 二创工程：承载 Creative Model。
	ProjectTypeCreative ProjectType = "CREATIVE"
)

// Valid 判断类型是否为合法枚举值。
func (t ProjectType) Valid() bool {
	return t == ProjectTypeOriginal || t == ProjectTypeCreative
}

// ProjectStatus 是工程状态。
type ProjectStatus string

const (
	ProjectStatusDraft    ProjectStatus = "DRAFT"
	ProjectStatusActive   ProjectStatus = "ACTIVE"
	ProjectStatusArchived ProjectStatus = "ARCHIVED"
)

// Valid 判断状态是否为合法枚举值。
func (s ProjectStatus) Valid() bool {
	switch s {
	case ProjectStatusDraft, ProjectStatusActive, ProjectStatusArchived:
		return true
	default:
		return false
	}
}

// 领域错误：由 api 层翻译为 HTTP 错误码。
var (
	ErrProjectNameRequired = errors.New("项目名称不能为空")
	ErrProjectNameTooLong  = errors.New("项目名称不能超过 200 个字符")
	ErrProjectTypeInvalid  = errors.New("项目类型非法")
	ErrProjectStatusBad    = errors.New("项目状态非法")
	// ErrProjectNotFound 由 repository 在查不到记录时返回，api 层翻译为 404。
	ErrProjectNotFound = errors.New("项目不存在")
)

// Project 是工程实体。
// 说明：ID 使用 UUID 字符串；DeletedAt 非空表示已软删除（SPEC.md §25）。
type Project struct {
	ID          string
	Name        string
	Description string
	Type        ProjectType
	Status      ProjectStatus
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
}

// Validate 校验工程实体的业务约束。
func (p *Project) Validate() error {
	name := strings.TrimSpace(p.Name)
	if name == "" {
		return ErrProjectNameRequired
	}
	if len([]rune(name)) > 200 {
		return ErrProjectNameTooLong
	}
	if !p.Type.Valid() {
		return ErrProjectTypeInvalid
	}
	if p.Status != "" && !p.Status.Valid() {
		return ErrProjectStatusBad
	}
	return nil
}

// Normalize 清理输入：去空白、补默认值。写库前必须调用。
func (p *Project) Normalize() {
	p.Name = strings.TrimSpace(p.Name)
	p.Description = strings.TrimSpace(p.Description)
	if p.Status == "" {
		p.Status = ProjectStatusActive
	}
}
