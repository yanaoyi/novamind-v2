package parser

import (
	"strings"
	"testing"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

// buildSample 生成一份带章节标题的中文样本。
func buildSample() string {
	parts := []string{
		"《测试原著》",
		"",
		"序章 起点",
		"",
		"这是序章的内容，用于验证开篇章节被正确保留。",
		"",
		"第一章 初遇",
		"",
		"林默站在月台上，风把她的头发吹得凌乱。",
		"她说：“我们回不去了。”",
		"",
		"第二章 裂痕",
		"",
		"三年后，一切都变了。",
		"这里提到了“第一章”这个词，但它出现在正文中间，不应该被当成标题。",
		"",
		"第三卷 终局",
		"",
		"最后一段正文。",
	}
	return strings.Join(parts, "\n")
}

func TestSplitChaptersChinese(t *testing.T) {
	text := buildSample()
	chapters := SplitChapters(text)

	wantTitles := []string{"开篇", "序章 起点", "第一章 初遇", "第二章 裂痕", "第三卷 终局"}
	got := titles(chapters)
	if len(got) != len(wantTitles) {
		t.Fatalf("应切出 %d 章，实际 %d：%v", len(wantTitles), len(got), got)
	}
	for i := range wantTitles {
		if got[i] != wantTitles[i] {
			t.Errorf("第 %d 章标题应为 %q，实际 %q", i+1, wantTitles[i], got[i])
		}
	}

	for i, c := range chapters {
		if c.No != i+1 {
			t.Errorf("章节号应连续，第 %d 项为 %d", i+1, c.No)
		}
		if strings.TrimSpace(c.Content) == "" {
			t.Errorf("第 %d 章内容为空", c.No)
		}
		if c.Start < 0 || c.End <= c.Start || c.End > int64(len(text)) {
			t.Errorf("第 %d 章位置区间非法: [%d,%d)", c.No, c.Start, c.End)
		}
	}

	// 正文里出现的“第一章”不应被误判为标题
	if strings.Contains(chapters[3].Title, "第一章") {
		t.Error("正文中的“第一章”被误判成标题")
	}
}

func TestSplitChaptersNoMarkersFallsBackToChunks(t *testing.T) {
	text := strings.Repeat("这是一段没有任何章节标题的正文。", 1000) // 约 1.7 万字
	chapters := SplitChapters(text)

	if len(chapters) < 2 {
		t.Fatalf("超长无标题正文应被兜底切分，实际 %d 章", len(chapters))
	}
	for _, c := range chapters {
		if n := utf8.RuneCountInString(c.Content); n > DefaultChunkRunes {
			t.Errorf("第 %d 段超过兜底粒度：%d 字", c.No, n)
		}
	}
	if !strings.Contains(chapters[0].Title, "自动切分") {
		t.Errorf("兜底切分的标题应带标记，实际 %q", chapters[0].Title)
	}
}

func TestSplitChaptersEmpty(t *testing.T) {
	if got := SplitChapters("   \n\n  "); len(got) != 0 {
		t.Fatalf("空白文本应切出 0 章，实际 %d", len(got))
	}
}

func TestDecodeTextUTF8(t *testing.T) {
	text, enc, err := DecodeText([]byte(buildSample()))
	if err != nil {
		t.Fatalf("解码失败: %v", err)
	}
	if enc != "UTF-8" {
		t.Errorf("应识别为 UTF-8，实际 %s", enc)
	}
	if !strings.Contains(text, "第三卷 终局") {
		t.Errorf("解码内容不正确，前 30 字: %q", firstRunes(text, 30))
	}
}

func TestDecodeTextGB18030(t *testing.T) {
	original := buildSample()
	encoded, _, err := transform.Bytes(simplifiedchinese.GB18030.NewEncoder(), []byte(original))
	if err != nil {
		t.Fatalf("生成 GB18030 样本失败: %v", err)
	}
	if utf8.Valid(encoded) {
		t.Skip("样本恰好是合法 UTF-8，跳过（不影响结论）")
	}

	text, enc, err := DecodeText(encoded)
	if err != nil {
		t.Fatalf("解码 GB18030 失败: %v", err)
	}
	if enc != "GB18030" {
		t.Errorf("应识别为 GB18030，实际 %s", enc)
	}
	if text != original {
		t.Errorf("GB18030 解码结果与原文不一致\n原文: %q\n解码: %q", firstRunes(original, 40), firstRunes(text, 40))
	}
}

func TestDecodeTextBOM(t *testing.T) {
	withBOM := append([]byte{0xEF, 0xBB, 0xBF}, []byte("第一章 起")...)
	text, enc, err := DecodeText(withBOM)
	if err != nil {
		t.Fatalf("解码失败: %v", err)
	}
	if enc != "UTF-8(BOM)" {
		t.Errorf("应识别为 UTF-8(BOM)，实际 %s", enc)
	}
	if text != "第一章 起" {
		t.Errorf("BOM 未被去除，实际 %q", text)
	}
}

func TestDecodeTextEmpty(t *testing.T) {
	if _, _, err := DecodeText([]byte("   \n  ")); err != ErrEmptyText {
		t.Fatalf("空内容应返回 ErrEmptyText，实际 %v", err)
	}
}

func TestParseByFilenameUnsupported(t *testing.T) {
	// 只有文件头、没有任何对象的 PDF 应当报错（而不是静默返回空文本）
	if _, _, err := ParseByFilename("a.pdf", []byte("%PDF-1.4")); err == nil {
		t.Error("残缺 PDF 应返回错误")
	}
	if _, _, err := ParseByFilename("a.doc", []byte("x")); err == nil {
		t.Error(".doc 应返回不支持")
	}
	if _, _, err := ParseByFilename("a.epub", []byte("x")); err == nil {
		t.Error(".epub 应返回不支持")
	}
}

func titles(chapters []Chapter) []string {
	out := make([]string, 0, len(chapters))
	for _, c := range chapters {
		out = append(out, c.Title)
	}
	return out
}

func firstRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
