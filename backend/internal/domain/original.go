package domain

import (
	"errors"
	"strings"
	"time"
)

// SourceType 是原著来源类型（规格书 §8.1 source_type）。
type SourceType string

const (
	SourceTypeManual SourceType = "MANUAL"
	SourceTypeTXT    SourceType = "TXT"
	SourceTypeDOCX   SourceType = "DOCX"
	SourceTypePDF    SourceType = "PDF"
)

// Valid 判断来源类型是否合法。
func (t SourceType) Valid() bool {
	switch t {
	case SourceTypeManual, SourceTypeTXT, SourceTypeDOCX, SourceTypePDF:
		return true
	default:
		return false
	}
}

// OriginalStatus 是原著处理状态。
type OriginalStatus string

const (
	// OriginalStatusDraft 已创建但尚未导入正文。
	OriginalStatusDraft OriginalStatus = "DRAFT"
	// OriginalStatusParsed 已导入并完成章节切分。
	OriginalStatusParsed OriginalStatus = "PARSED"
	// OriginalStatusAnalyzed 已完成 AI 分析（Phase 3 起使用）。
	OriginalStatusAnalyzed OriginalStatus = "ANALYZED"
)

// Valid 判断状态是否合法。
func (s OriginalStatus) Valid() bool {
	switch s {
	case OriginalStatusDraft, OriginalStatusParsed, OriginalStatusAnalyzed:
		return true
	default:
		return false
	}
}

// 领域错误。
var (
	ErrOriginalTitleRequired   = errors.New("原著标题不能为空")
	ErrOriginalTitleTooLong    = errors.New("原著标题不能超过 200 个字符")
	ErrOriginalNotFound        = errors.New("原著不存在")
	ErrOriginalAlreadyExists   = errors.New("该工程已存在原著")
	ErrOriginalNotOriginalProj = errors.New("只有 ORIGINAL 类型的工程可以创建原著")
	ErrChapterNotFound         = errors.New("章节不存在")
	ErrImportSourceInvalid     = errors.New("导入文件类型不支持")
	ErrImportEmpty             = errors.New("导入文件没有可解析的正文")
	// ErrOriginalStatusInvalid 是原著状态非法错误。
	ErrOriginalStatusInvalid = errors.New("原著状态非法")
)

// OriginalWork 是原著作品。
type OriginalWork struct {
	ID           string
	ProjectID    string
	Title        string
	Author       string
	Description  string
	SourceType   SourceType
	SourceFileID *string
	Status       OriginalStatus
	CharCount    int64
	ChapterCount int
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    *time.Time
}

// Validate 校验原著实体。
func (w *OriginalWork) Validate() error {
	title := strings.TrimSpace(w.Title)
	if title == "" {
		return ErrOriginalTitleRequired
	}
	if len([]rune(title)) > 200 {
		return ErrOriginalTitleTooLong
	}
	if !w.SourceType.Valid() {
		return ErrImportSourceInvalid
	}
	if w.Status != "" && !w.Status.Valid() {
		return ErrOriginalStatusInvalid
	}
	return nil
}

// Normalize 清洗输入并补默认值。
func (w *OriginalWork) Normalize() {
	w.Title = strings.TrimSpace(w.Title)
	w.Author = strings.TrimSpace(w.Author)
	w.Description = strings.TrimSpace(w.Description)
	if w.SourceType == "" {
		w.SourceType = SourceTypeManual
	}
	if w.Status == "" {
		w.Status = OriginalStatusDraft
	}
}

// OriginalChapter 是原著章节。
type OriginalChapter struct {
	ID             string
	OriginalWorkID string
	ChapterNo      int
	Title          string
	Content        string
	Summary        string
	StartPosition  int64
	EndPosition    int64
	CharCount      int
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      *time.Time
}

// CharCountOf 统计正文字符数（按 rune 计，中文一字算一个）。
func CharCountOf(content string) int {
	return len([]rune(content))
}

// UploadedFile 是上传文件登记（规格书 §3.5 文件管理）。
type UploadedFile struct {
	ID           string
	ProjectID    *string
	OriginalName string
	StoredPath   string
	MimeType     string
	SizeBytes    int64
	SHA256       string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    *time.Time
}
