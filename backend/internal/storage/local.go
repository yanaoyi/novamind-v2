package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// LocalStore 是基于本地文件系统的实现。
type LocalStore struct {
	root string
}

// NewLocalStore 创建本地存储；root 不存在时自动创建。
func NewLocalStore(root string) (*LocalStore, error) {
	if strings.TrimSpace(root) == "" {
		return nil, errors.New("存储根目录不能为空")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("解析存储根目录失败: %w", err)
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, fmt.Errorf("创建存储根目录失败: %w", err)
	}
	return &LocalStore{root: abs}, nil
}

// Root 返回存储根的绝对路径（测试与排查用）。
func (s *LocalStore) Root() string { return s.root }

// Put 落盘并返回路径、大小、SHA256 与 MIME 类型。
func (s *LocalStore) Put(ctx context.Context, projectID string, originalName string, r io.Reader) (SavedFile, error) {
	if err := ctx.Err(); err != nil {
		return SavedFile{}, err
	}

	dir := filepath.Join(s.root, sanitizeSegment(projectID))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return SavedFile{}, fmt.Errorf("创建项目目录失败: %w", err)
	}

	name := uuid.NewString() + sanitizeExt(filepath.Ext(originalName))
	full := filepath.Join(dir, name)

	f, err := os.Create(full)
	if err != nil {
		return SavedFile{}, fmt.Errorf("创建文件失败: %w", err)
	}
	defer f.Close()

	// 先读 512 字节做 MIME 嗅探，再连同剩余内容一起写入
	head := make([]byte, 512)
	n, readErr := io.ReadFull(r, head)
	if readErr != nil && !errors.Is(readErr, io.EOF) && !errors.Is(readErr, io.ErrUnexpectedEOF) {
		_ = os.Remove(full)
		return SavedFile{}, fmt.Errorf("读取上传内容失败: %w", readErr)
	}
	head = head[:n]

	hasher := sha256.New()
	written, err := io.Copy(io.MultiWriter(f, hasher), io.MultiReader(strings.NewReader(string(head)), r))
	if err != nil {
		_ = os.Remove(full)
		return SavedFile{}, fmt.Errorf("写入文件失败: %w", err)
	}

	return SavedFile{
		StoredPath: filepath.Join(sanitizeSegment(projectID), name),
		SizeBytes:  written,
		SHA256:     hex.EncodeToString(hasher.Sum(nil)),
		MimeType:   http.DetectContentType(head),
	}, nil
}

// Open 打开已保存的文件。
func (s *LocalStore) Open(_ context.Context, storedPath string) (io.ReadCloser, error) {
	full, err := s.resolve(storedPath)
	if err != nil {
		return nil, err
	}
	return os.Open(full)
}

// Delete 删除文件（不存在视为成功）。
func (s *LocalStore) Delete(_ context.Context, storedPath string) error {
	full, err := s.resolve(storedPath)
	if err != nil {
		return err
	}
	if err := os.Remove(full); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// resolve 把相对路径解析为绝对路径，并确保没有跳出存储根。
func (s *LocalStore) resolve(storedPath string) (string, error) {
	cleaned := filepath.Clean(storedPath)
	if filepath.IsAbs(cleaned) || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("非法存储路径: %s", storedPath)
	}
	full := filepath.Join(s.root, cleaned)
	if !strings.HasPrefix(full, s.root+string(os.PathSeparator)) {
		return "", fmt.Errorf("非法存储路径: %s", storedPath)
	}
	return full, nil
}

// sanitizeSegment 只保留安全字符，防止路径穿越。
func sanitizeSegment(segment string) string {
	var b strings.Builder
	for _, r := range segment {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	if b.Len() == 0 {
		return "unknown"
	}
	return b.String()
}

// sanitizeExt 只允许简单扩展名，其余丢弃。
func sanitizeExt(ext string) string {
	if len(ext) < 2 || len(ext) > 10 {
		return ""
	}
	for _, r := range ext[1:] {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			return ""
		}
	}
	return strings.ToLower(ext)
}
