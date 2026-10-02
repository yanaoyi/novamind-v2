// Package storage 提供上传文件的存储抽象。
// Phase 2 使用本地文件系统（SPEC.md §23「文件：V1 本地文件系统」）；
// 生产可替换为 OSS / S3 / MinIO——接口不变，故 service 只依赖 Store。
package storage

import (
	"context"
	"io"
)

// SavedFile 是一次落盘的结果。
type SavedFile struct {
	// StoredPath 是相对存储根的路径，入库时存这个值（便于整体迁移存储根）
	StoredPath string
	SizeBytes  int64
	SHA256     string
	MimeType   string
}

// Store 是文件存储抽象。
type Store interface {
	// Put 保存文件；projectID 用于分目录，originalName 仅用于推断扩展名。
	Put(ctx context.Context, projectID string, originalName string, r io.Reader) (SavedFile, error)
	// Open 读取文件（供后续"重新解析"使用）。
	Open(ctx context.Context, storedPath string) (io.ReadCloser, error)
	// Delete 删除文件。
	Delete(ctx context.Context, storedPath string) error
}
