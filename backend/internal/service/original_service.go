package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/yanaoyi/novamindv2/backend/internal/domain"
	"github.com/yanaoyi/novamindv2/backend/internal/parser"
	"github.com/yanaoyi/novamindv2/backend/internal/storage"
)

// OriginalRepository 是原著 service 需要的仓储能力。
type OriginalRepository interface {
	CreateFile(ctx context.Context, f *domain.UploadedFile) error
	GetFile(ctx context.Context, id string) (*domain.UploadedFile, error)
	CreateWork(ctx context.Context, w *domain.OriginalWork) error
	GetWorkByID(ctx context.Context, id string) (*domain.OriginalWork, error)
	GetWorkByProject(ctx context.Context, projectID string) (*domain.OriginalWork, error)
	SetWorkSource(ctx context.Context, workID string, sourceType domain.SourceType, fileID string) error
	ReplaceChapters(ctx context.Context, workID string, chapters []domain.OriginalChapter, charCount int64) error
	ListChapters(ctx context.Context, workID string, page, pageSize int) ([]domain.OriginalChapter, int64, error)
	GetChapter(ctx context.Context, workID string, chapterNo int) (*domain.OriginalChapter, error)
}

// Reparse 用已保存的源文件重新解析章节（用于导入规则升级后重跑）。
//
// 典型用途：章节切分规则改进后，不需要让作者重新上传，直接重跑即可；
// 实现上通过 report 回调把阶段与进度交给调用方（异步任务据此更新 tasks.progress）。
func (s *OriginalService) Reparse(ctx context.Context, workID string, report func(stage string, percent int)) (*ImportResult, error) {
	work, err := s.repo.GetWorkByID(ctx, workID)
	if err != nil {
		return nil, err
	}
	if work.SourceFileID == nil || strings.TrimSpace(*work.SourceFileID) == "" {
		return nil, errors.New("该原著没有源文件，无法重新解析（可能是手工录入的）")
	}
	if report != nil {
		report("读取源文件", 10)
	}
	file, err := s.repo.GetFile(ctx, *work.SourceFileID)
	if err != nil {
		return nil, err
	}
	rc, err := s.files.Open(ctx, file.StoredPath)
	if err != nil {
		return nil, fmt.Errorf("打开源文件失败: %w", err)
	}
	defer rc.Close()

	data, err := readLimited(rc, s.maxUpload)
	if err != nil {
		return nil, err
	}
	if report != nil {
		report("解析文本与章节", 40)
	}

	text, encoding, err := parser.ParseByFilename(file.OriginalName, data)
	if err != nil {
		switch {
		case errors.Is(err, parser.ErrEmptyText):
			return nil, domain.ErrImportEmpty
		case errors.Is(err, parser.ErrUnsupportedFormat),
			errors.Is(err, parser.ErrPDFNoTextLayer),
			errors.Is(err, parser.ErrPDFEncrypted):
			return nil, fmt.Errorf("%w：%v", domain.ErrImportSourceInvalid, err)
		default:
			return nil, err
		}
	}
	chapters := parser.SplitChapters(text)
	if len(chapters) == 0 {
		return nil, domain.ErrImportEmpty
	}
	if report != nil {
		report("写入章节", 70)
	}

	domainChapters := make([]domain.OriginalChapter, 0, len(chapters))
	briefs := make([]ChapterBrief, 0, len(chapters))
	for _, c := range chapters {
		charCount := domain.CharCountOf(c.Content)
		domainChapters = append(domainChapters, domain.OriginalChapter{
			ChapterNo:     c.No,
			Title:         c.Title,
			Content:       c.Content,
			StartPosition: c.Start,
			EndPosition:   c.End,
			CharCount:     charCount,
		})
		briefs = append(briefs, ChapterBrief{ChapterNo: c.No, Title: c.Title, CharCount: charCount})
	}

	charCount := int64(utf8.RuneCountInString(text))
	if err := s.repo.ReplaceChapters(ctx, workID, domainChapters, charCount); err != nil {
		return nil, err
	}
	if report != nil {
		report("完成", 100)
	}
	return &ImportResult{
		FileID: file.ID, FileName: file.OriginalName, SizeBytes: file.SizeBytes,
		Encoding: encoding, CharCount: charCount, ChapterCount: len(domainChapters), Chapters: briefs,
	}, nil
}

// OriginalService 是原著业务服务。
type OriginalService struct {
	repo      OriginalRepository
	projects  ProjectRepository
	files     storage.Store
	maxUpload int64
}

// NewOriginalService 构建服务。
func NewOriginalService(repo OriginalRepository, projects ProjectRepository, files storage.Store, maxUploadBytes int64) *OriginalService {
	return &OriginalService{repo: repo, projects: projects, files: files, maxUpload: maxUploadBytes}
}

// CreateOriginalInput 是创建原著的入参。
type CreateOriginalInput struct {
	ProjectID   string
	Title       string
	Author      string
	Description string
}

// Create 为指定工程创建原著。
// 约束：工程必须是 ORIGINAL 类型；一个工程只能有一部原著。
func (s *OriginalService) Create(ctx context.Context, in CreateOriginalInput) (*domain.OriginalWork, error) {
	project, err := s.projects.GetByID(ctx, in.ProjectID)
	if err != nil {
		return nil, err
	}
	if project.Type != domain.ProjectTypeOriginal {
		return nil, domain.ErrOriginalNotOriginalProj
	}

	if _, err := s.repo.GetWorkByProject(ctx, in.ProjectID); err == nil {
		return nil, domain.ErrOriginalAlreadyExists
	} else if !errors.Is(err, domain.ErrOriginalNotFound) {
		return nil, err
	}

	w := &domain.OriginalWork{
		ProjectID:   in.ProjectID,
		Title:       in.Title,
		Author:      in.Author,
		Description: in.Description,
		SourceType:  domain.SourceTypeManual,
	}
	w.Normalize()
	if err := w.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.CreateWork(ctx, w); err != nil {
		return nil, err
	}
	return w, nil
}

// Get 按 ID 取原著。
func (s *OriginalService) Get(ctx context.Context, id string) (*domain.OriginalWork, error) {
	if strings.TrimSpace(id) == "" {
		return nil, domain.ErrOriginalNotFound
	}
	return s.repo.GetWorkByID(ctx, id)
}

// ImportResult 是导入结果摘要。
type ImportResult struct {
	FileID       string
	FileName     string
	SizeBytes    int64
	Encoding     string
	CharCount    int64
	ChapterCount int
	Chapters     []ChapterBrief
}

// ChapterBrief 是导入结果里的章节摘要（只带标题与字数，避免响应过大）。
type ChapterBrief struct {
	ChapterNo int    `json:"chapter_no"`
	Title     string `json:"title"`
	CharCount int    `json:"char_count"`
}

// ErrUploadTooLarge 表示上传超过大小上限。
var ErrUploadTooLarge = errors.New("上传文件过大")

// Import 导入原文：落盘 → 解析（编码探测）→ 章节切分 → 事务写入。
// 幂等：同一部原著重复导入会整体替换章节，不会重复累积。
func (s *OriginalService) Import(ctx context.Context, workID, fileName string, r io.Reader) (*ImportResult, error) {
	work, err := s.repo.GetWorkByID(ctx, workID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(fileName) == "" {
		return nil, fmt.Errorf("缺少文件名，无法判断格式")
	}

	data, err := readLimited(r, s.maxUpload)
	if err != nil {
		return nil, err
	}

	text, encoding, err := parser.ParseByFilename(fileName, data)
	if err != nil {
		// 把解析层错误收敛成领域错误，避免 api 层依赖 parser 包的细节
		switch {
		case errors.Is(err, parser.ErrEmptyText):
			return nil, domain.ErrImportEmpty
		case errors.Is(err, parser.ErrUnsupportedFormat),
			errors.Is(err, parser.ErrPDFNoTextLayer),
			errors.Is(err, parser.ErrPDFEncrypted):
			return nil, fmt.Errorf("%w：%v", domain.ErrImportSourceInvalid, err)
		default:
			return nil, err
		}
	}
	chapters := parser.SplitChapters(text)
	if len(chapters) == 0 {
		return nil, domain.ErrImportEmpty
	}

	saved, err := s.files.Put(ctx, work.ProjectID, fileName, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	file := &domain.UploadedFile{
		ProjectID:    &work.ProjectID,
		OriginalName: fileName,
		StoredPath:   saved.StoredPath,
		MimeType:     saved.MimeType,
		SizeBytes:    saved.SizeBytes,
		SHA256:       saved.SHA256,
	}
	if err := s.repo.CreateFile(ctx, file); err != nil {
		_ = s.files.Delete(ctx, saved.StoredPath)
		return nil, err
	}

	domainChapters := make([]domain.OriginalChapter, 0, len(chapters))
	briefs := make([]ChapterBrief, 0, len(chapters))
	for _, c := range chapters {
		charCount := domain.CharCountOf(c.Content)
		domainChapters = append(domainChapters, domain.OriginalChapter{
			ChapterNo:     c.No,
			Title:         c.Title,
			Content:       c.Content,
			StartPosition: c.Start,
			EndPosition:   c.End,
			CharCount:     charCount,
		})
		briefs = append(briefs, ChapterBrief{ChapterNo: c.No, Title: c.Title, CharCount: charCount})
	}

	charCount := int64(utf8.RuneCountInString(text))
	if err := s.repo.ReplaceChapters(ctx, workID, domainChapters, charCount); err != nil {
		// 章节写入失败则删掉刚落盘的文件，避免留下孤儿文件
		_ = s.files.Delete(ctx, saved.StoredPath)
		return nil, err
	}

	sourceType := domain.SourceType(strings.ToUpper(strings.TrimPrefix(fileName[strings.LastIndex(fileName, ".")+1:], ".")))
	if !sourceType.Valid() || sourceType == domain.SourceTypeManual {
		sourceType = domain.SourceTypeTXT
	}
	if err := s.repo.SetWorkSource(ctx, workID, sourceType, file.ID); err != nil {
		return nil, err
	}

	return &ImportResult{
		FileID:       file.ID,
		FileName:     fileName,
		SizeBytes:    saved.SizeBytes,
		Encoding:     encoding,
		CharCount:    charCount,
		ChapterCount: len(domainChapters),
		Chapters:     briefs,
	}, nil
}

// ListChapters 分页取章节目录。
func (s *OriginalService) ListChapters(ctx context.Context, workID string, page, pageSize int) ([]domain.OriginalChapter, int64, error) {
	if _, err := s.repo.GetWorkByID(ctx, workID); err != nil {
		return nil, 0, err
	}
	items, total, err := s.repo.ListChapters(ctx, workID, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	if items == nil {
		items = []domain.OriginalChapter{}
	}
	return items, total, nil
}

// GetChapter 取单章（含正文）。
func (s *OriginalService) GetChapter(ctx context.Context, workID string, chapterNo int) (*domain.OriginalChapter, error) {
	if chapterNo <= 0 {
		return nil, domain.ErrChapterNotFound
	}
	return s.repo.GetChapter(ctx, workID, chapterNo)
}

// readLimited 读取至多 limit 字节，超限直接报错（避免把大文件整个读进内存）。
func readLimited(r io.Reader, limit int64) ([]byte, error) {
	if limit <= 0 {
		limit = 50 * 1024 * 1024
	}
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, fmt.Errorf("读取上传内容失败: %w", err)
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%w：上限 %d MB", ErrUploadTooLarge, limit/(1024*1024))
	}
	return data, nil
}
