package parser

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
)

// buildDocx 在内存里造一个最小可用的 .docx。
func buildDocx(t *testing.T) []byte {
	t.Helper()
	const document = `<?xml version="1.0" encoding="UTF-8"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p><w:r><w:t>第一章 初遇</w:t></w:r></w:p>
    <w:p><w:r><w:t>林默说：</w:t></w:r><w:r><w:t>我们回不去了。</w:t></w:r></w:p>
    <w:p><w:r><w:t>第二章 裂痕</w:t></w:r></w:p>
  </w:body>
</w:document>`

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("word/document.xml")
	if err != nil {
		t.Fatalf("创建 docx 条目失败: %v", err)
	}
	if _, err := w.Write([]byte(document)); err != nil {
		t.Fatalf("写入 docx 失败: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("关闭 docx 失败: %v", err)
	}
	return buf.Bytes()
}

func TestParseDOCXExtractsParagraphs(t *testing.T) {
	text, err := ParseDOCX(buildDocx(t))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}

	want := "第一章 初遇\n林默说：我们回不去了。\n第二章 裂痕"
	if text != want {
		t.Fatalf("docx 抽取结果不符\n期望: %q\n实际: %q", want, text)
	}

	// 抽出的文本应能继续做章节切分
	chapters := SplitChapters(text)
	if len(chapters) != 2 {
		t.Fatalf("应切出 2 章，实际 %d", len(chapters))
	}
	if !strings.HasPrefix(chapters[0].Title, "第一章") {
		t.Errorf("第一章标题不正确: %q", chapters[0].Title)
	}
}

func TestParseByFilenameDOCX(t *testing.T) {
	text, enc, err := ParseByFilename("原著.docx", buildDocx(t))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if enc != "UTF-8(DOCX)" {
		t.Errorf("编码标记应为 UTF-8(DOCX)，实际 %s", enc)
	}
	if !strings.Contains(text, "第二章 裂痕") {
		t.Errorf("内容缺失: %q", text)
	}
}

func TestParseDOCXInvalid(t *testing.T) {
	if _, err := ParseDOCX([]byte("这不是一个 zip")); err == nil {
		t.Error("非法 docx 应报错")
	}

	// 合法 zip 但缺少 word/document.xml
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	if _, err := zw.Create("hello.txt"); err != nil {
		t.Fatalf("构造 zip 失败: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("关闭 zip 失败: %v", err)
	}
	if _, err := ParseDOCX(buf.Bytes()); err == nil {
		t.Error("缺少 document.xml 应报错")
	}
}
