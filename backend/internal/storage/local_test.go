package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalStorePutOpenDelete(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	store, err := NewLocalStore(root)
	if err != nil {
		t.Fatalf("创建存储失败: %v", err)
	}

	content := []byte("第一章 测试\n这是正文。")
	saved, err := store.Put(ctx, "proj-1", "原著.TXT", bytes.NewReader(content))
	if err != nil {
		t.Fatalf("落盘失败: %v", err)
	}

	if saved.SizeBytes != int64(len(content)) {
		t.Errorf("大小不符：期望 %d，实际 %d", len(content), saved.SizeBytes)
	}
	sum := sha256.Sum256(content)
	if saved.SHA256 != hex.EncodeToString(sum[:]) {
		t.Errorf("SHA256 不符: %s", saved.SHA256)
	}
	if !strings.HasPrefix(saved.StoredPath, "proj-1/") {
		t.Errorf("应按项目分目录，实际 %s", saved.StoredPath)
	}
	if !strings.HasSuffix(saved.StoredPath, ".txt") {
		t.Errorf("扩展名应统一小写，实际 %s", saved.StoredPath)
	}

	// 文件确实落在存储根下
	if _, err := os.Stat(filepath.Join(root, saved.StoredPath)); err != nil {
		t.Fatalf("文件不存在: %v", err)
	}

	rc, err := store.Open(ctx, saved.StoredPath)
	if err != nil {
		t.Fatalf("读取失败: %v", err)
	}
	got, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		t.Fatalf("读取内容失败: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("内容不一致: %q", got)
	}

	if err := store.Delete(ctx, saved.StoredPath); err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	if _, err := store.Open(ctx, saved.StoredPath); err == nil {
		t.Error("删除后不应还能打开")
	}
	// 重复删除不报错
	if err := store.Delete(ctx, saved.StoredPath); err != nil {
		t.Errorf("重复删除应幂等，实际 %v", err)
	}
}

func TestLocalStoreRejectsPathTraversal(t *testing.T) {
	ctx := context.Background()
	store, err := NewLocalStore(t.TempDir())
	if err != nil {
		t.Fatalf("创建存储失败: %v", err)
	}

	for _, bad := range []string{"../../etc/passwd", "/etc/passwd", "../secret"} {
		if _, err := store.Open(ctx, bad); err == nil {
			t.Errorf("路径穿越 %q 应被拒绝", bad)
		}
		if err := store.Delete(ctx, bad); err == nil {
			t.Errorf("删除路径穿越 %q 应被拒绝", bad)
		}
	}
}

func TestLocalStoreSanitizesFileName(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	store, err := NewLocalStore(root)
	if err != nil {
		t.Fatalf("创建存储失败: %v", err)
	}

	// 恶意文件名：既想穿越目录，又带非法扩展名
	saved, err := store.Put(ctx, "../../evil", "../../../x.sh", bytes.NewReader([]byte("data")))
	if err != nil {
		t.Fatalf("落盘失败: %v", err)
	}
	if strings.Contains(saved.StoredPath, "..") {
		t.Fatalf("落盘路径不应包含 ..：%s", saved.StoredPath)
	}
	if !strings.HasPrefix(saved.StoredPath, "______evil/") {
		t.Errorf("项目目录应被清洗，实际 %s", saved.StoredPath)
	}
	// 文件名本体是 UUID，扩展名保留 .sh（在存储根内无害）
	if _, err := os.Stat(filepath.Join(root, saved.StoredPath)); err != nil {
		t.Fatalf("文件应落在存储根内: %v", err)
	}
}

func TestNewLocalStoreRequiresRoot(t *testing.T) {
	if _, err := NewLocalStore("  "); err == nil {
		t.Error("空存储根应报错")
	}
}
