package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/yanaoyi/novamindv2/backend/internal/domain"
)

// ---------- 持久化模型 ----------

type fileModel struct {
	ID           string         `gorm:"column:id;type:uuid;primaryKey"`
	ProjectID    *string        `gorm:"column:project_id;type:uuid"`
	OriginalName string         `gorm:"column:original_name;size:300;not null"`
	StoredPath   string         `gorm:"column:stored_path;not null"`
	MimeType     string         `gorm:"column:mime_type;size:120;not null;default:''"`
	SizeBytes    int64          `gorm:"column:size_bytes;not null;default:0"`
	SHA256       string         `gorm:"column:sha256;size:64;not null;default:''"`
	CreatedAt    time.Time      `gorm:"column:created_at;not null"`
	UpdatedAt    time.Time      `gorm:"column:updated_at;not null"`
	DeletedAt    gorm.DeletedAt `gorm:"column:deleted_at;index"`
}

func (fileModel) TableName() string { return "files" }

type originalWorkModel struct {
	ID           string         `gorm:"column:id;type:uuid;primaryKey"`
	ProjectID    string         `gorm:"column:project_id;type:uuid;not null"`
	Title        string         `gorm:"column:title;size:200;not null"`
	Author       string         `gorm:"column:author;size:120;not null;default:''"`
	Description  string         `gorm:"column:description;not null;default:''"`
	SourceType   string         `gorm:"column:source_type;size:20;not null;default:MANUAL"`
	SourceFileID *string        `gorm:"column:source_file_id;type:uuid"`
	Status       string         `gorm:"column:status;size:20;not null;default:DRAFT"`
	CharCount    int64          `gorm:"column:char_count;not null;default:0"`
	ChapterCount int            `gorm:"column:chapter_count;not null;default:0"`
	CreatedAt    time.Time      `gorm:"column:created_at;not null"`
	UpdatedAt    time.Time      `gorm:"column:updated_at;not null"`
	DeletedAt    gorm.DeletedAt `gorm:"column:deleted_at;index"`
}

func (originalWorkModel) TableName() string { return "original_works" }

type originalChapterModel struct {
	ID             string         `gorm:"column:id;type:uuid;primaryKey"`
	OriginalWorkID string         `gorm:"column:original_work_id;type:uuid;not null"`
	ChapterNo      int            `gorm:"column:chapter_no;not null"`
	Title          string         `gorm:"column:title;size:300;not null;default:''"`
	Content        string         `gorm:"column:content;not null;default:''"`
	Summary        string         `gorm:"column:summary;not null;default:''"`
	StartPosition  int64          `gorm:"column:start_position;not null;default:0"`
	EndPosition    int64          `gorm:"column:end_position;not null;default:0"`
	CharCount      int            `gorm:"column:char_count;not null;default:0"`
	CreatedAt      time.Time      `gorm:"column:created_at;not null"`
	UpdatedAt      time.Time      `gorm:"column:updated_at;not null"`
	DeletedAt      gorm.DeletedAt `gorm:"column:deleted_at;index"`
}

func (originalChapterModel) TableName() string { return "original_chapters" }

// ---------- 仓储 ----------

// OriginalRepo 是原著相关仓储。
type OriginalRepo struct {
	db *gorm.DB
}

// NewOriginalRepo 构建仓储。
func NewOriginalRepo(db *gorm.DB) *OriginalRepo { return &OriginalRepo{db: db} }

// CreateFile 登记上传文件。
func (r *OriginalRepo) CreateFile(ctx context.Context, f *domain.UploadedFile) error {
	if f.ID == "" {
		id, err := uuid.NewV7()
		if err != nil {
			return fmt.Errorf("生成文件 ID 失败: %w", err)
		}
		f.ID = id.String()
	}
	now := time.Now().UTC()
	f.CreatedAt, f.UpdatedAt = now, now

	m := fileModel{
		ID:           f.ID,
		ProjectID:    f.ProjectID,
		OriginalName: f.OriginalName,
		StoredPath:   f.StoredPath,
		MimeType:     f.MimeType,
		SizeBytes:    f.SizeBytes,
		SHA256:       f.SHA256,
		CreatedAt:    f.CreatedAt,
		UpdatedAt:    f.UpdatedAt,
	}
	if err := r.db.WithContext(ctx).Create(&m).Error; err != nil {
		return fmt.Errorf("登记文件失败: %w", err)
	}
	return nil
}

// GetFile 按 ID 取上传文件登记。
func (r *OriginalRepo) GetFile(ctx context.Context, id string) (*domain.UploadedFile, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, errors.New("文件不存在")
	}
	var m fileModel
	if err := r.db.WithContext(ctx).First(&m, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("文件不存在")
		}
		return nil, fmt.Errorf("查询文件失败: %w", err)
	}
	f := domain.UploadedFile{
		ID: m.ID, ProjectID: m.ProjectID, OriginalName: m.OriginalName,
		StoredPath: m.StoredPath, MimeType: m.MimeType, SizeBytes: m.SizeBytes, SHA256: m.SHA256,
		CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}
	if m.DeletedAt.Valid {
		t := m.DeletedAt.Time
		f.DeletedAt = &t
	}
	return &f, nil
}

// CreateWork 创建原著。
func (r *OriginalRepo) CreateWork(ctx context.Context, w *domain.OriginalWork) error {
	if w.ID == "" {
		id, err := uuid.NewV7()
		if err != nil {
			return fmt.Errorf("生成原著 ID 失败: %w", err)
		}
		w.ID = id.String()
	}
	now := time.Now().UTC()
	w.CreatedAt, w.UpdatedAt = now, now

	m := originalWorkModel{
		ID:           w.ID,
		ProjectID:    w.ProjectID,
		Title:        w.Title,
		Author:       w.Author,
		Description:  w.Description,
		SourceType:   string(w.SourceType),
		SourceFileID: w.SourceFileID,
		Status:       string(w.Status),
		CharCount:    w.CharCount,
		ChapterCount: w.ChapterCount,
		CreatedAt:    w.CreatedAt,
		UpdatedAt:    w.UpdatedAt,
	}
	if err := r.db.WithContext(ctx).Create(&m).Error; err != nil {
		return fmt.Errorf("创建原著失败: %w", err)
	}
	return nil
}

// SetWorkSource 记录原著来源（导入的文件类型与文件 ID）。
func (r *OriginalRepo) SetWorkSource(ctx context.Context, workID string, sourceType domain.SourceType, fileID string) error {
	if _, err := uuid.Parse(workID); err != nil {
		return domain.ErrOriginalNotFound
	}
	updates := map[string]any{
		"source_type": string(sourceType),
		"updated_at":  time.Now().UTC(),
	}
	if fileID != "" {
		updates["source_file_id"] = fileID
	}
	res := r.db.WithContext(ctx).Model(&originalWorkModel{}).Where("id = ?", workID).Updates(updates)
	if res.Error != nil {
		return fmt.Errorf("更新原著来源失败: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.ErrOriginalNotFound
	}
	return nil
}

// GetWorkByID 按 ID 查原著（未删除）。
func (r *OriginalRepo) GetWorkByID(ctx context.Context, id string) (*domain.OriginalWork, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, domain.ErrOriginalNotFound
	}
	var m originalWorkModel
	if err := r.db.WithContext(ctx).First(&m, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrOriginalNotFound
		}
		return nil, fmt.Errorf("查询原著失败: %w", err)
	}
	w := toDomainWork(m)
	return &w, nil
}

// GetWorkByProject 按文章查原著。
func (r *OriginalRepo) GetWorkByProject(ctx context.Context, projectID string) (*domain.OriginalWork, error) {
	if _, err := uuid.Parse(projectID); err != nil {
		return nil, domain.ErrOriginalNotFound
	}
	var m originalWorkModel
	if err := r.db.WithContext(ctx).First(&m, "project_id = ?", projectID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrOriginalNotFound
		}
		return nil, fmt.Errorf("查询原著失败: %w", err)
	}
	w := toDomainWork(m)
	return &w, nil
}

// ReplaceChapters 用新切分结果整体替换某原著的章节（单事务）：
//  1. 软删除旧章节
//  2. 批量插入新章节
//  3. 回写原著的章节数 / 字数 / 状态
//
// 幂等性由"先删后插 + 唯一索引（未删除行）"共同保证：重复导入同一本书不会产生重复章节。
func (r *OriginalRepo) ReplaceChapters(ctx context.Context, workID string, chapters []domain.OriginalChapter, charCount int64) error {
	if _, err := uuid.Parse(workID); err != nil {
		return domain.ErrOriginalNotFound
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var exists int64
		if err := tx.Model(&originalWorkModel{}).Where("id = ?", workID).Count(&exists).Error; err != nil {
			return fmt.Errorf("校验原著失败: %w", err)
		}
		if exists == 0 {
			return domain.ErrOriginalNotFound
		}

		if err := tx.Where("original_work_id = ?", workID).Delete(&originalChapterModel{}).Error; err != nil {
			return fmt.Errorf("清理旧章节失败: %w", err)
		}

		now := time.Now().UTC()
		models := make([]originalChapterModel, 0, len(chapters))
		for _, c := range chapters {
			id, err := uuid.NewV7()
			if err != nil {
				return fmt.Errorf("生成章节 ID 失败: %w", err)
			}
			models = append(models, originalChapterModel{
				ID:             id.String(),
				OriginalWorkID: workID,
				ChapterNo:      c.ChapterNo,
				Title:          c.Title,
				Content:        c.Content,
				Summary:        c.Summary,
				StartPosition:  c.StartPosition,
				EndPosition:    c.EndPosition,
				CharCount:      c.CharCount,
				CreatedAt:      now,
				UpdatedAt:      now,
			})
		}
		if len(models) > 0 {
			if err := tx.CreateInBatches(&models, 200).Error; err != nil {
				return fmt.Errorf("写入章节失败: %w", err)
			}
		}

		res := tx.Model(&originalWorkModel{}).Where("id = ?", workID).Updates(map[string]any{
			"chapter_count": len(models),
			"char_count":    charCount,
			"status":        string(domain.OriginalStatusParsed),
			"updated_at":    now,
		})
		if res.Error != nil {
			return fmt.Errorf("更新原著统计失败: %w", res.Error)
		}
		return nil
	})
}

// ListChapters 分页查询章节目录（不返回正文，避免目录接口拖大响应）。
func (r *OriginalRepo) ListChapters(ctx context.Context, workID string, page, pageSize int) ([]domain.OriginalChapter, int64, error) {
	if _, err := uuid.Parse(workID); err != nil {
		return nil, 0, domain.ErrOriginalNotFound
	}

	query := r.db.WithContext(ctx).Model(&originalChapterModel{}).Where("original_work_id = ?", workID)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("统计章节失败: %w", err)
	}

	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 50
	}
	if pageSize > 200 {
		pageSize = 200
	}

	var models []originalChapterModel
	if err := query.
		Select("id, original_work_id, chapter_no, title, summary, start_position, end_position, char_count, created_at, updated_at, deleted_at").
		Order("chapter_no ASC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&models).Error; err != nil {
		return nil, 0, fmt.Errorf("查询章节失败: %w", err)
	}

	out := make([]domain.OriginalChapter, 0, len(models))
	for _, m := range models {
		out = append(out, toDomainChapter(m))
	}
	return out, total, nil
}

// UpdateChapterSummary 回写章节摘要（AI 提案审核通过时使用）。
func (r *OriginalRepo) UpdateChapterSummary(ctx context.Context, workID string, chapterNo int, summary string) error {
	if _, err := uuid.Parse(workID); err != nil {
		return domain.ErrOriginalNotFound
	}
	res := r.db.WithContext(ctx).Model(&originalChapterModel{}).
		Where("original_work_id = ? AND chapter_no = ?", workID, chapterNo).
		Updates(map[string]any{"summary": summary, "updated_at": time.Now().UTC()})
	if res.Error != nil {
		return fmt.Errorf("更新章节摘要失败: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.ErrChapterNotFound
	}
	return nil
}

// ListChapterContents 取章节（含正文），按章节号升序，最多 limit 章。
// 供 AI 分析阶段组装上下文；limit 用于控制单次送入模型的篇幅。
func (r *OriginalRepo) ListChapterContents(ctx context.Context, workID string, limit int) ([]domain.OriginalChapter, error) {
	if _, err := uuid.Parse(workID); err != nil {
		return nil, domain.ErrOriginalNotFound
	}
	if limit <= 0 {
		limit = 50
	}
	var models []originalChapterModel
	if err := r.db.WithContext(ctx).
		Where("original_work_id = ?", workID).
		Order("chapter_no ASC").Limit(limit).Find(&models).Error; err != nil {
		return nil, fmt.Errorf("查询章节正文失败: %w", err)
	}
	out := make([]domain.OriginalChapter, 0, len(models))
	for _, m := range models {
		out = append(out, toDomainChapter(m))
	}
	return out, nil
}

// GetChapter 按章节号取单章（含正文）。
func (r *OriginalRepo) GetChapter(ctx context.Context, workID string, chapterNo int) (*domain.OriginalChapter, error) {
	if _, err := uuid.Parse(workID); err != nil {
		return nil, domain.ErrOriginalNotFound
	}
	var m originalChapterModel
	if err := r.db.WithContext(ctx).
		First(&m, "original_work_id = ? AND chapter_no = ?", workID, chapterNo).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrChapterNotFound
		}
		return nil, fmt.Errorf("查询章节失败: %w", err)
	}
	c := toDomainChapter(m)
	return &c, nil
}

func toDomainWork(m originalWorkModel) domain.OriginalWork {
	w := domain.OriginalWork{
		ID:           m.ID,
		ProjectID:    m.ProjectID,
		Title:        m.Title,
		Author:       m.Author,
		Description:  m.Description,
		SourceType:   domain.SourceType(m.SourceType),
		SourceFileID: m.SourceFileID,
		Status:       domain.OriginalStatus(m.Status),
		CharCount:    m.CharCount,
		ChapterCount: m.ChapterCount,
		CreatedAt:    m.CreatedAt,
		UpdatedAt:    m.UpdatedAt,
	}
	if m.DeletedAt.Valid {
		t := m.DeletedAt.Time
		w.DeletedAt = &t
	}
	return w
}

func toDomainChapter(m originalChapterModel) domain.OriginalChapter {
	c := domain.OriginalChapter{
		ID:             m.ID,
		OriginalWorkID: m.OriginalWorkID,
		ChapterNo:      m.ChapterNo,
		Title:          m.Title,
		Content:        m.Content,
		Summary:        m.Summary,
		StartPosition:  m.StartPosition,
		EndPosition:    m.EndPosition,
		CharCount:      m.CharCount,
		CreatedAt:      m.CreatedAt,
		UpdatedAt:      m.UpdatedAt,
	}
	if m.DeletedAt.Valid {
		t := m.DeletedAt.Time
		c.DeletedAt = &t
	}
	return c
}
