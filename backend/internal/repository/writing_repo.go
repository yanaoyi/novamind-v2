package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/yanaoyi/novamindv2/backend/internal/domain"
)

type volumeModel struct {
	ID             string         `gorm:"column:id;type:uuid;primaryKey"`
	CreativeWorkID string         `gorm:"column:creative_work_id;type:uuid;not null"`
	Title          string         `gorm:"column:title;size:200;not null"`
	Summary        string         `gorm:"column:summary;not null;default:''"`
	Sequence       int            `gorm:"column:sequence;not null"`
	CreatedAt      time.Time      `gorm:"column:created_at;not null"`
	UpdatedAt      time.Time      `gorm:"column:updated_at;not null"`
	DeletedAt      gorm.DeletedAt `gorm:"column:deleted_at;index"`
}

func (volumeModel) TableName() string { return "creative_volumes" }

type chapterModel struct {
	ID             string         `gorm:"column:id;type:uuid;primaryKey"`
	CreativeWorkID string         `gorm:"column:creative_work_id;type:uuid;not null"`
	VolumeID       *string        `gorm:"column:volume_id;type:uuid"`
	OutlineNodeID  *string        `gorm:"column:outline_node_id;type:uuid"`
	ChapterNo      int            `gorm:"column:chapter_no;not null"`
	Title          string         `gorm:"column:title;size:200;not null"`
	Summary        string         `gorm:"column:summary;not null;default:''"`
	Content        string         `gorm:"column:content;not null;default:''"`
	Status         string         `gorm:"column:status;size:10;not null"`
	WordCount      int            `gorm:"column:word_count;not null"`
	Purpose        string         `gorm:"column:purpose;not null;default:''"`
	Conflict       string         `gorm:"column:conflict;not null;default:''"`
	Outcome        string         `gorm:"column:outcome;not null;default:''"`
	CreatedAt      time.Time      `gorm:"column:created_at;not null"`
	UpdatedAt      time.Time      `gorm:"column:updated_at;not null"`
	DeletedAt      gorm.DeletedAt `gorm:"column:deleted_at;index"`
}

func (chapterModel) TableName() string { return "creative_chapters" }

type sceneModel struct {
	ID            string         `gorm:"column:id;type:uuid;primaryKey"`
	ChapterID     string         `gorm:"column:chapter_id;type:uuid;not null"`
	Sequence      int            `gorm:"column:sequence;not null"`
	Title         string         `gorm:"column:title;size:200;not null;default:''"`
	Location      string         `gorm:"column:location;size:200;not null;default:''"`
	Characters    string         `gorm:"column:characters;type:jsonb;not null;default:'[]'"`
	Purpose       string         `gorm:"column:purpose;not null;default:''"`
	Conflict      string         `gorm:"column:conflict;not null;default:''"`
	EmotionalGoal string         `gorm:"column:emotional_goal;not null;default:''"`
	Content       string         `gorm:"column:content;not null;default:''"`
	CreatedAt     time.Time      `gorm:"column:created_at;not null"`
	UpdatedAt     time.Time      `gorm:"column:updated_at;not null"`
	DeletedAt     gorm.DeletedAt `gorm:"column:deleted_at;index"`
}

func (sceneModel) TableName() string { return "creative_scenes" }

type chapterVersionModel struct {
	ID        string    `gorm:"column:id;type:uuid;primaryKey"`
	ChapterID string    `gorm:"column:chapter_id;type:uuid;not null"`
	VersionNo int       `gorm:"column:version_no;not null"`
	Content   string    `gorm:"column:content;not null;default:''"`
	WordCount int       `gorm:"column:word_count;not null"`
	Note      string    `gorm:"column:note;size:200;not null;default:''"`
	CreatedAt time.Time `gorm:"column:created_at;not null"`
}

func (chapterVersionModel) TableName() string { return "chapter_versions" }

type issueModel struct {
	ID             string         `gorm:"column:id;type:uuid;primaryKey"`
	CreativeWorkID string         `gorm:"column:creative_work_id;type:uuid;not null"`
	ChapterID      *string        `gorm:"column:chapter_id;type:uuid"`
	Severity       string         `gorm:"column:severity;size:10;not null"`
	Type           string         `gorm:"column:type;size:30;not null"`
	Description    string         `gorm:"column:description;not null"`
	Evidence       string         `gorm:"column:evidence;not null;default:''"`
	Suggestion     string         `gorm:"column:suggestion;not null;default:''"`
	Status         string         `gorm:"column:status;size:10;not null"`
	CreatedAt      time.Time      `gorm:"column:created_at;not null"`
	UpdatedAt      time.Time      `gorm:"column:updated_at;not null"`
	DeletedAt      gorm.DeletedAt `gorm:"column:deleted_at;index"`
}

func (issueModel) TableName() string { return "consistency_issues" }

// WritingRepo 是写作系统仓储（卷 / 章节 / 场景 / 版本 / 一致性）。
type WritingRepo struct {
	db *gorm.DB
}

// NewWritingRepo 构建仓储。
func NewWritingRepo(db *gorm.DB) *WritingRepo { return &WritingRepo{db: db} }

// ---------- 卷 ----------

// CreateVolume 新增卷。
func (r *WritingRepo) CreateVolume(ctx context.Context, v *domain.CreativeVolume) error {
	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("生成卷 ID 失败: %w", err)
	}
	v.ID = id.String()
	now := time.Now().UTC()
	v.CreatedAt, v.UpdatedAt = now, now
	if err := r.db.WithContext(ctx).Create(&volumeModel{
		ID: v.ID, CreativeWorkID: v.CreativeWorkID, Title: v.Title, Summary: v.Summary,
		Sequence: v.Sequence, CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		if isForeignKeyViolation(err) {
			return domain.ErrCreativeNotFound
		}
		return fmt.Errorf("创建卷失败: %w", err)
	}
	return nil
}

// ListVolumes 列出卷。
func (r *WritingRepo) ListVolumes(ctx context.Context, workID string) ([]domain.CreativeVolume, error) {
	var models []volumeModel
	if err := r.db.WithContext(ctx).Where("creative_work_id = ?", workID).Order("sequence ASC").Find(&models).Error; err != nil {
		return nil, fmt.Errorf("查询卷失败: %w", err)
	}
	out := make([]domain.CreativeVolume, 0, len(models))
	for _, m := range models {
		out = append(out, domain.CreativeVolume{
			ID: m.ID, CreativeWorkID: m.CreativeWorkID, Title: m.Title, Summary: m.Summary,
			Sequence: m.Sequence, CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
		})
	}
	return out, nil
}

// ---------- 章节 ----------

// CreateChapter 新增章节。
func (r *WritingRepo) CreateChapter(ctx context.Context, c *domain.CreativeChapter) error {
	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("生成章节 ID 失败: %w", err)
	}
	c.ID = id.String()
	now := time.Now().UTC()
	c.CreatedAt, c.UpdatedAt = now, now
	m := chapterModel{
		ID: c.ID, CreativeWorkID: c.CreativeWorkID, VolumeID: c.VolumeID, ChapterNo: c.ChapterNo,
		Title: c.Title, Summary: c.Summary, Content: c.Content, Status: string(c.Status),
		WordCount: c.WordCount, Purpose: c.Purpose, Conflict: c.Conflict, Outcome: c.Outcome,
		OutlineNodeID: c.OutlineNodeID,
		CreatedAt:     now, UpdatedAt: now,
	}
	if err := r.db.WithContext(ctx).Create(&m).Error; err != nil {
		if isUniqueViolation(err) {
			return errors.New("该章节号已存在")
		}
		if isForeignKeyViolation(err) {
			return domain.ErrCreativeNotFound
		}
		return fmt.Errorf("创建章节失败: %w", err)
	}
	return nil
}

// GetChapter 取章节。
func (r *WritingRepo) GetChapter(ctx context.Context, id string) (*domain.CreativeChapter, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, domain.ErrCreativeChapterNotFound
	}
	var m chapterModel
	if err := r.db.WithContext(ctx).First(&m, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrCreativeChapterNotFound
		}
		return nil, fmt.Errorf("查询章节失败: %w", err)
	}
	c := toDomainCreativeChapter(m)
	return &c, nil
}

// ListChapters 列出章节（不含正文，供目录使用）。
func (r *WritingRepo) ListChapters(ctx context.Context, workID string, withContent bool) ([]domain.CreativeChapter, error) {
	query := r.db.WithContext(ctx).Model(&chapterModel{}).Where("creative_work_id = ?", workID)
	if !withContent {
		query = query.Select("id, creative_work_id, volume_id, outline_node_id, chapter_no, title, summary, status, word_count, purpose, conflict, outcome, created_at, updated_at, deleted_at, '' as content")
	}
	var models []chapterModel
	if err := query.Order("chapter_no ASC").Find(&models).Error; err != nil {
		return nil, fmt.Errorf("查询章节失败: %w", err)
	}
	out := make([]domain.CreativeChapter, 0, len(models))
	for _, m := range models {
		out = append(out, toDomainCreativeChapter(m))
	}
	return out, nil
}

// UpdateChapter 更新章节（含大纲信息；正文更新时由 service 负责生成版本）。
func (r *WritingRepo) UpdateChapter(ctx context.Context, c *domain.CreativeChapter) error {
	now := time.Now().UTC()
	res := r.db.WithContext(ctx).Model(&chapterModel{}).Where("id = ?", c.ID).Updates(map[string]any{
		"volume_id": c.VolumeID, "chapter_no": c.ChapterNo, "title": c.Title, "summary": c.Summary,
		"content": c.Content, "status": string(c.Status), "word_count": c.WordCount,
		"purpose": c.Purpose, "conflict": c.Conflict, "outcome": c.Outcome, "updated_at": now,
	})
	if res.Error != nil {
		if isUniqueViolation(res.Error) {
			return errors.New("该章节号已存在")
		}
		return fmt.Errorf("更新章节失败: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		if _, err := r.GetChapter(ctx, c.ID); err != nil {
			return err
		}
	}
	c.UpdatedAt = now
	return nil
}

// DeleteChapter 软删除章节。
func (r *WritingRepo) DeleteChapter(ctx context.Context, id string) error {
	if _, err := uuid.Parse(id); err != nil {
		return domain.ErrCreativeChapterNotFound
	}
	res := r.db.WithContext(ctx).Where("id = ?", id).Delete(&chapterModel{})
	if res.Error != nil {
		return fmt.Errorf("删除章节失败: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.ErrCreativeChapterNotFound
	}
	return nil
}

// ---------- 版本 ----------

// NextVersionNo 取下一个版本号。
func (r *WritingRepo) NextVersionNo(ctx context.Context, chapterID string) (int, error) {
	var maxNo *int
	if err := r.db.WithContext(ctx).Model(&chapterVersionModel{}).
		Where("chapter_id = ?", chapterID).Select("MAX(version_no)").Scan(&maxNo).Error; err != nil {
		return 0, fmt.Errorf("查询版本号失败: %w", err)
	}
	if maxNo == nil {
		return 1, nil
	}
	return *maxNo + 1, nil
}

// CreateVersion 保存一个版本。
func (r *WritingRepo) CreateVersion(ctx context.Context, v *domain.ChapterVersion) error {
	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("生成版本 ID 失败: %w", err)
	}
	v.ID = id.String()
	v.CreatedAt = time.Now().UTC()
	if err := r.db.WithContext(ctx).Create(&chapterVersionModel{
		ID: v.ID, ChapterID: v.ChapterID, VersionNo: v.VersionNo, Content: v.Content,
		WordCount: v.WordCount, Note: v.Note, CreatedAt: v.CreatedAt,
	}).Error; err != nil {
		return fmt.Errorf("保存版本失败: %w", err)
	}
	return nil
}

// ListVersions 列出章节版本（不含正文，供列表展示）。
func (r *WritingRepo) ListVersions(ctx context.Context, chapterID string) ([]domain.ChapterVersion, error) {
	var models []chapterVersionModel
	if err := r.db.WithContext(ctx).Where("chapter_id = ?", chapterID).
		Select("id, chapter_id, version_no, word_count, note, created_at, '' as content").
		Order("version_no DESC").Find(&models).Error; err != nil {
		return nil, fmt.Errorf("查询版本失败: %w", err)
	}
	out := make([]domain.ChapterVersion, 0, len(models))
	for _, m := range models {
		out = append(out, toDomainVersion(m))
	}
	return out, nil
}

// GetVersion 取某个版本（含正文）。
func (r *WritingRepo) GetVersion(ctx context.Context, chapterID string, versionNo int) (*domain.ChapterVersion, error) {
	var m chapterVersionModel
	if err := r.db.WithContext(ctx).
		First(&m, "chapter_id = ? AND version_no = ?", chapterID, versionNo).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrChapterVersionNotFound
		}
		return nil, fmt.Errorf("查询版本失败: %w", err)
	}
	v := toDomainVersion(m)
	return &v, nil
}

// ---------- 场景 ----------

// CreateScene 新增场景。
func (r *WritingRepo) CreateScene(ctx context.Context, s *domain.CreativeScene) error {
	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("生成场景 ID 失败: %w", err)
	}
	s.ID = id.String()
	now := time.Now().UTC()
	s.CreatedAt, s.UpdatedAt = now, now
	characters, _ := json.Marshal(s.Characters)
	if err := r.db.WithContext(ctx).Create(&sceneModel{
		ID: s.ID, ChapterID: s.ChapterID, Sequence: s.Sequence, Title: s.Title, Location: s.Location,
		Characters: string(characters), Purpose: s.Purpose, Conflict: s.Conflict,
		EmotionalGoal: s.EmotionalGoal, Content: s.Content, CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		if isForeignKeyViolation(err) {
			return domain.ErrCreativeChapterNotFound
		}
		return fmt.Errorf("创建场景失败: %w", err)
	}
	return nil
}

// ListScenes 列出章节下的场景。
func (r *WritingRepo) ListScenes(ctx context.Context, chapterID string) ([]domain.CreativeScene, error) {
	if _, err := uuid.Parse(chapterID); err != nil {
		return nil, domain.ErrCreativeChapterNotFound
	}
	var models []sceneModel
	if err := r.db.WithContext(ctx).Where("chapter_id = ?", chapterID).Order("sequence ASC").Find(&models).Error; err != nil {
		return nil, fmt.Errorf("查询场景失败: %w", err)
	}
	out := make([]domain.CreativeScene, 0, len(models))
	for _, m := range models {
		out = append(out, toDomainScene(m))
	}
	return out, nil
}

// ---------- 一致性 ----------

// CreateIssues 批量写入一致性问题。
func (r *WritingRepo) CreateIssues(ctx context.Context, issues []domain.ConsistencyIssue) (int, error) {
	if len(issues) == 0 {
		return 0, nil
	}
	models := make([]issueModel, 0, len(issues))
	now := time.Now().UTC()
	for i := range issues {
		issue := &issues[i]
		issue.Normalize()
		if err := issue.Validate(); err != nil {
			continue
		}
		id, err := uuid.NewV7()
		if err != nil {
			return 0, fmt.Errorf("生成问题 ID 失败: %w", err)
		}
		issue.ID = id.String()
		issue.CreatedAt, issue.UpdatedAt = now, now
		models = append(models, issueModel{
			ID: issue.ID, CreativeWorkID: issue.CreativeWorkID, ChapterID: issue.ChapterID,
			Severity: issue.Severity, Type: issue.Type, Description: issue.Description,
			Evidence: issue.Evidence, Suggestion: issue.Suggestion, Status: issue.Status,
			CreatedAt: now, UpdatedAt: now,
		})
	}
	if len(models) == 0 {
		return 0, nil
	}
	if err := r.db.WithContext(ctx).Create(&models).Error; err != nil {
		return 0, fmt.Errorf("写入一致性问题失败: %w", err)
	}
	return len(models), nil
}

// ListIssues 列出一致性问题。
func (r *WritingRepo) ListIssues(ctx context.Context, workID, status string) ([]domain.ConsistencyIssue, error) {
	query := r.db.WithContext(ctx).Model(&issueModel{}).Where("creative_work_id = ?", workID)
	if status != "" {
		query = query.Where("status = ?", status)
	}
	var models []issueModel
	if err := query.Order("created_at DESC").Find(&models).Error; err != nil {
		return nil, fmt.Errorf("查询一致性问题失败: %w", err)
	}
	out := make([]domain.ConsistencyIssue, 0, len(models))
	for _, m := range models {
		out = append(out, toDomainIssue(m))
	}
	return out, nil
}

// UpdateIssueStatus 更新问题状态（解决 / 忽略）。
func (r *WritingRepo) UpdateIssueStatus(ctx context.Context, id, status string) error {
	if _, err := uuid.Parse(id); err != nil {
		return domain.ErrIssueNotFound
	}
	res := r.db.WithContext(ctx).Model(&issueModel{}).Where("id = ?", id).
		Updates(map[string]any{"status": status, "updated_at": time.Now().UTC()})
	if res.Error != nil {
		return fmt.Errorf("更新问题状态失败: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.ErrIssueNotFound
	}
	return nil
}

func toDomainCreativeChapter(m chapterModel) domain.CreativeChapter {
	return domain.CreativeChapter{
		ID: m.ID, CreativeWorkID: m.CreativeWorkID, VolumeID: m.VolumeID,
		OutlineNodeID: m.OutlineNodeID, ChapterNo: m.ChapterNo,
		Title: m.Title, Summary: m.Summary, Content: m.Content, Status: domain.ChapterStatus(m.Status),
		WordCount: m.WordCount, Purpose: m.Purpose, Conflict: m.Conflict, Outcome: m.Outcome,
		CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}
}

func toDomainVersion(m chapterVersionModel) domain.ChapterVersion {
	return domain.ChapterVersion{
		ID: m.ID, ChapterID: m.ChapterID, VersionNo: m.VersionNo, Content: m.Content,
		WordCount: m.WordCount, Note: m.Note, CreatedAt: m.CreatedAt,
	}
}

func toDomainScene(m sceneModel) domain.CreativeScene {
	s := domain.CreativeScene{
		ID: m.ID, ChapterID: m.ChapterID, Sequence: m.Sequence, Title: m.Title, Location: m.Location,
		Purpose: m.Purpose, Conflict: m.Conflict, EmotionalGoal: m.EmotionalGoal, Content: m.Content,
		CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt, Characters: []string{},
	}
	unmarshalJSONB("creative_scenes", "characters", m.Characters, &s.Characters)
	return s
}

func toDomainIssue(m issueModel) domain.ConsistencyIssue {
	return domain.ConsistencyIssue{
		ID: m.ID, CreativeWorkID: m.CreativeWorkID, ChapterID: m.ChapterID,
		Severity: m.Severity, Type: m.Type, Description: m.Description,
		Evidence: m.Evidence, Suggestion: m.Suggestion, Status: m.Status,
		CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}
}
