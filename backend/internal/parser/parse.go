package parser

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// 解析相关错误。
var (
	// ErrUnsupportedFormat 表示扩展名不在支持范围内。
	ErrUnsupportedFormat = errors.New("不支持的文件格式")
	// ErrPDFNotImplemented PDF 解析尚未实现（见 CODEX_STATE 已知缺口）。
	ErrPDFNotImplemented = errors.New("PDF 解析尚未实现")
)

// ParseByFilename 按扩展名把上传内容解析成纯文本，返回文本与实际使用的编码。
// 支持：TXT/MD（自动探测编码）、DOCX。
func ParseByFilename(filename string, data []byte) (text string, encoding string, err error) {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".txt", ".text", ".md", "":
		return DecodeText(data)
	case ".docx":
		raw, err := ParseDOCX(data)
		if err != nil {
			return "", "", err
		}
		if strings.TrimSpace(raw) == "" {
			return "", "", ErrEmptyText
		}
		return raw, "UTF-8(DOCX)", nil
	case ".pdf":
		return "", "", fmt.Errorf("%w：当前仅支持 TXT / DOCX，PDF 解析计划在 Phase 2 收尾前补上", ErrPDFNotImplemented)
	case ".doc":
		return "", "", fmt.Errorf("%w：.doc 旧格式请先另存为 .docx", ErrUnsupportedFormat)
	default:
		return "", "", fmt.Errorf("%w：%s", ErrUnsupportedFormat, ext)
	}
}
