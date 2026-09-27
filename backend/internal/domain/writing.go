package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// 写作与一致性的领域错误。
var (
	ErrVolumeNotFound          = errors.New("卷不存在")
	ErrVolumeTitleEmpty        = errors.New("卷标题不能为空")
	ErrCreativeChapterNotFound = errors.New("章节不存在")
	ErrChapterTitleEmpty       = errors.New("章节标题不能为空")
	ErrChapterNoInvalid        = errors.New("章节号必须为正整数")
	ErrChapterStatusInvalid    = errors.New("章节状态非法")
	ErrChapterVersionNotFound  = errors.New("章节版本不存在")
	ErrSceneNotFound           = errors.New("场景不存在")
	ErrIssueNotFound           = errors.New("一致性问题不存在")
	ErrIssueSeverityInvalid    = errors.New("严重程度非法")
	ErrIssueTypeInvalid        = errors.New("问题类型非法")
	ErrIssueStatusInvalid      = errors.New("问题状态非法")
	ErrExportFormatInvalid     = errors.New("导出格式不支持")
)

// ChapterStatus 是章节状态（规格书 §28）。
type ChapterStatus string

const (
	ChapterDraft  ChapterStatus = "DRAFT"
	ChapterReview ChapterStatus = "REVIEW"
	ChapterFinal  ChapterStatus = "FINAL"
)

// Valid 判断章节状态是否合法。
func (s ChapterStatus) Valid() bool {
	return s == ChapterDraft || s == ChapterReview || s == ChapterFinal
}

// CreativeVolume 是卷（规格书 §27 大纲的第一层）。
type CreativeVolume struct {
	ID             string
	CreativeWorkID string
	Title          string
	Summary        string
	Sequence       int
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      *time.Time
}

// Normalize 清洗输入。
func (v *CreativeVolume) Normalize() {
	v.Title = strings.TrimSpace(v.Title)
	v.Summary = strings.TrimSpace(v.Summary)
	if v.Sequence == 0 {
		v.Sequence = 1
	}
}

// Validate 校验卷。
func (v *CreativeVolume) Validate() error {
	if v.Title == "" {
		return ErrVolumeTitleEmpty
	}
	return nil
}

// CreativeChapter 是二创章节（含大纲信息，规格书 §27-§28）。
type CreativeChapter struct {
	ID             string
	CreativeWorkID string
	VolumeID       *string
	ChapterNo      int
	Title          string
	Summary        string
	Content        string
	Status         ChapterStatus
	WordCount      int
	Purpose        string
	Conflict       string
	Outcome        string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      *time.Time
}

// Normalize 清洗输入并补默认值。
func (c *CreativeChapter) Normalize() {
	c.Title = strings.TrimSpace(c.Title)
	c.Summary = strings.TrimSpace(c.Summary)
	c.Purpose = strings.TrimSpace(c.Purpose)
	c.Conflict = strings.TrimSpace(c.Conflict)
	c.Outcome = strings.TrimSpace(c.Outcome)
	if c.Status == "" {
		c.Status = ChapterDraft
	}
	c.WordCount = CountWords(c.Content)
}

// Validate 校验章节。
func (c *CreativeChapter) Validate() error {
	if strings.TrimSpace(c.Title) == "" {
		return ErrChapterTitleEmpty
	}
	if c.ChapterNo <= 0 {
		return ErrChapterNoInvalid
	}
	if !c.Status.Valid() {
		return fmt.Errorf("%w: %s", ErrChapterStatusInvalid, c.Status)
	}
	return nil
}

// CountWords 统计正文字数（中文按字符计，去掉空白）。
func CountWords(content string) int {
	count := 0
	for _, r := range content {
		if r == ' ' || r == '\n' || r == '\t' || r == '\r' {
			continue
		}
		count++
	}
	return count
}

// ChapterVersion 是章节版本（规格书 §59）。
type ChapterVersion struct {
	ID        string
	ChapterID string
	VersionNo int
	Content   string
	WordCount int
	Note      string
	CreatedAt time.Time
}

// CreativeScene 是场景（规格书 §29）。
type CreativeScene struct {
	ID            string
	ChapterID     string
	Sequence      int
	Title         string
	Location      string
	Characters    []string
	Purpose       string
	Conflict      string
	EmotionalGoal string
	Content       string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	DeletedAt     *time.Time
}

// Normalize 清洗输入。
func (s *CreativeScene) Normalize() {
	s.Title = strings.TrimSpace(s.Title)
	s.Location = strings.TrimSpace(s.Location)
	s.Purpose = strings.TrimSpace(s.Purpose)
	s.Conflict = strings.TrimSpace(s.Conflict)
	s.EmotionalGoal = strings.TrimSpace(s.EmotionalGoal)
	if s.Sequence == 0 {
		s.Sequence = 1
	}
	if s.Characters == nil {
		s.Characters = []string{}
	}
}

// ConsistencyIssue 是一致性检查发现的问题（规格书 §39）。
type ConsistencyIssue struct {
	ID             string
	CreativeWorkID string
	ChapterID      *string
	Severity       string
	Type           string
	Description    string
	Evidence       string
	Suggestion     string
	Status         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      *time.Time
}

// Normalize 清洗输入并补默认值。
func (i *ConsistencyIssue) Normalize() {
	i.Severity = strings.ToLower(strings.TrimSpace(i.Severity))
	i.Type = strings.ToLower(strings.TrimSpace(i.Type))
	i.Description = strings.TrimSpace(i.Description)
	i.Evidence = strings.TrimSpace(i.Evidence)
	i.Suggestion = strings.TrimSpace(i.Suggestion)
	if i.Severity == "" {
		i.Severity = "medium"
	}
	if i.Status == "" {
		i.Status = "OPEN"
	}
}

// Validate 校验问题。
func (i *ConsistencyIssue) Validate() error {
	switch i.Severity {
	case "high", "medium", "low":
	default:
		return fmt.Errorf("%w: %s", ErrIssueSeverityInvalid, i.Severity)
	}
	switch i.Type {
	case "character", "world", "timeline", "plot", "language":
	default:
		return fmt.Errorf("%w: %s", ErrIssueTypeInvalid, i.Type)
	}
	switch i.Status {
	case "OPEN", "RESOLVED", "IGNORED":
	default:
		return fmt.Errorf("%w: %s", ErrIssueStatusInvalid, i.Status)
	}
	if i.Description == "" {
		return errors.New("问题描述不能为空")
	}
	return nil
}
