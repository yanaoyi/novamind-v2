package parser

import (
	"bytes"
	"compress/zlib"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// ---------- 测试用 PDF 拼装工具 ----------

type pdfObjSpec struct {
	num  int
	body string // 字典，或「字典 stream...endstream」
}

// buildPDF 拼一个结构完整的 PDF（含 xref/trailer），尽量贴近真实文件。
func buildPDF(objs []pdfObjSpec) []byte {
	var buf bytes.Buffer
	buf.WriteString("%PDF-1.4\n%\xe2\xe3\xcf\xd3\n")
	offsets := map[int]int{}
	for _, o := range objs {
		offsets[o.num] = buf.Len()
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", o.num, o.body)
	}
	xref := buf.Len()
	maxNum := 0
	for _, o := range objs {
		if o.num > maxNum {
			maxNum = o.num
		}
	}
	fmt.Fprintf(&buf, "xref\n0 %d\n", maxNum+1)
	buf.WriteString("0000000000 65535 f \n")
	for i := 1; i <= maxNum; i++ {
		if off, ok := offsets[i]; ok {
			fmt.Fprintf(&buf, "%010d 00000 n \n", off)
		} else {
			buf.WriteString("0000000000 65535 f \n")
		}
	}
	fmt.Fprintf(&buf, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", maxNum+1, xref)
	return buf.Bytes()
}

func deflate(t *testing.T, raw string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)
	if _, err := w.Write([]byte(raw)); err != nil {
		t.Fatalf("压缩失败: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("压缩失败: %v", err)
	}
	return buf.Bytes()
}

func streamObj(body string) string {
	return streamObjWith(body, "")
}

func streamObjWith(body, extraDict string) string {
	return fmt.Sprintf("<< /Length %d%s >>\nstream\n%sendstream", len(body), extraDict, body)
}

// simpleFontPDF：WinAnsi 简单字体 + 未压缩内容流。
func simpleFontPDF() []byte {
	content := "BT /F1 24 Tf 72 720 Td (Hello PDF World) Tj ET\n" +
		"BT /F1 12 Tf 72 680 Td (Second line here) Tj ET\n"
	return buildPDF([]pdfObjSpec{
		{1, "<< /Type /Catalog /Pages 2 0 R >>"},
		{2, "<< /Type /Pages /Kids [3 0 R] /Count 1 >>"},
		{3, "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /Font << /F1 5 0 R >> >> /Contents 4 0 R >>"},
		{4, streamObj(content)},
		{5, "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>"},
	})
}

func TestParsePDFSimpleFont(t *testing.T) {
	text, err := ParsePDF(simpleFontPDF())
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if !strings.Contains(text, "Hello PDF World") {
		t.Errorf("缺少第一行文本: %q", text)
	}
	if !strings.Contains(text, "Second line here") {
		t.Errorf("缺少第二行文本: %q", text)
	}
	if !strings.Contains(text, "\n") {
		t.Errorf("不同的 Td 应该产生换行: %q", text)
	}
}

func TestParsePDFFlateAndHexStrings(t *testing.T) {
	content := "BT /F1 12 Tf 72 700 Td [(Compre) -30 (ssed) -30 (text)] TJ ET\n" +
		"BT /F1 12 Tf 72 660 Td <48656C6C6F> Tj ET\n"
	compressed := deflate(t, content)
	pdf := buildPDF([]pdfObjSpec{
		{1, "<< /Type /Catalog /Pages 2 0 R >>"},
		{2, "<< /Type /Pages /Kids [3 0 R] /Count 1 >>"},
		{3, "<< /Type /Page /Parent 2 0 R /Resources << /Font << /F1 5 0 R >> >> /Contents 4 0 R >>"},
		{4, streamObjWith(string(compressed), " /Filter /FlateDecode")},
		{5, "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>"},
	})
	text, err := ParsePDF(pdf)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if !strings.Contains(text, "Compressedtext") {
		t.Errorf("TJ 数组拼接不正确: %q", text)
	}
	if !strings.Contains(text, "Hello") {
		t.Errorf("十六进制字符串未解出: %q", text)
	}
}

func TestParsePDFKerningArray(t *testing.T) {
	content := "BT /F1 12 Tf 72 700 Td [(A) -250 (B) -20 (C)] TJ ET\n"
	pdf := buildPDF([]pdfObjSpec{
		{1, "<< /Type /Catalog /Pages 2 0 R >>"},
		{2, "<< /Type /Pages /Kids [3 0 R] /Count 1 >>"},
		{3, "<< /Type /Page /Parent 2 0 R /Resources << /Font << /F1 5 0 R >> >> /Contents 4 0 R >>"},
		{4, streamObj(content)},
		{5, "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>"},
	})
	text, err := ParsePDF(pdf)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if strings.TrimSpace(text) != "ABC" {
		t.Errorf("字距处理不对，期望 ABC，实际 %q", text)
	}
}

// CID 字体（Type0/Identity-H）+ ToUnicode：中文能不能抽出来全靠这一步。
func TestParsePDFCIDWithToUnicode(t *testing.T) {
	content := "BT /F1 12 Tf 72 700 Td <4E2D6587> Tj ET\n" +
		"BT /F1 12 Tf 72 660 Td <4E004E014E02> Tj ET\n"
	cmap := "/CIDInit /ProcSet findresource begin\n" +
		"12 dict begin\nbegincmap\n" +
		"1 begincodespacerange\n<0000> <FFFF>\nendcodespacerange\n" +
		"2 beginbfchar\n<4E2D> <4E2D>\n<6587> <6587>\nendbfchar\n" +
		"1 beginbfrange\n<4E00> <4E02> <4E00>\nendbfrange\n" +
		"endcmap\nend end"

	pdf := buildPDF([]pdfObjSpec{
		{1, "<< /Type /Catalog /Pages 2 0 R >>"},
		{2, "<< /Type /Pages /Kids [3 0 R] /Count 1 >>"},
		{3, "<< /Type /Page /Parent 2 0 R /Resources << /Font << /F1 5 0 R >> >> /Contents 4 0 R >>"},
		{4, streamObj(content)},
		{5, "<< /Type /Font /Subtype /Type0 /BaseFont /SimSun /Encoding /Identity-H /ToUnicode 6 0 R >>"},
		{6, streamObj(cmap)},
	})
	text, err := ParsePDF(pdf)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if !strings.Contains(text, "中文") {
		t.Errorf("bfchar 映射没生效: %q", text)
	}
	if !strings.Contains(text, "一丁丂") {
		t.Errorf("bfrange 映射没生效: %q", text)
	}
}

// 对象流（ObjStm）：页面字典与字体字典压在对象流里，只有内容流是顶层对象。
func TestParsePDFObjectStream(t *testing.T) {
	content := "BT /F1 12 Tf 72 700 Td (Inside object stream) Tj ET\n"
	pageDict := "<< /Type /Page /Parent 3 0 R /Resources << /Font << /F1 5 0 R >> >> /Contents 4 0 R >>"
	fontDict := "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>"
	// 对象流头部是「对象号 偏移」对，偏移相对 First
	header := fmt.Sprintf("2 0 5 %d ", len(pageDict))
	compressed := deflate(t, header+pageDict+fontDict)

	pdf := buildPDF([]pdfObjSpec{
		{1, "<< /Type /Catalog /Pages 3 0 R >>"},
		{3, "<< /Type /Pages /Kids [2 0 R] /Count 1 >>"},
		{4, streamObj(content)},
		// 对象流必须占用一个「顶层里不存在」的对象号，否则会与内层对象撞号
		{10, streamObjWith(string(compressed), fmt.Sprintf(" /Type /ObjStm /N 2 /First %d /Filter /FlateDecode", len(header)))},
	})
	text, err := ParsePDF(pdf)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if !strings.Contains(text, "Inside object stream") {
		t.Errorf("对象流里的页字典没被用上: %q", text)
	}
}

// Form XObject 里的文本也要抽出来。
func TestParsePDFFormXObject(t *testing.T) {
	form := "BT /F1 12 Tf 10 700 Td (Text in form) Tj ET\n"
	page := "BT /F1 12 Tf 72 760 Td (Text in page) Tj ET\n/FRM Do\n"
	pdf := buildPDF([]pdfObjSpec{
		{1, "<< /Type /Catalog /Pages 2 0 R >>"},
		{2, "<< /Type /Pages /Kids [3 0 R] /Count 1 >>"},
		{3, "<< /Type /Page /Parent 2 0 R /Resources << /Font << /F1 6 0 R >> /XObject << /FRM 5 0 R >> >> /Contents 4 0 R >>"},
		{4, streamObj(page)},
		{5, streamObjWith(form, " /Type /XObject /Subtype /Form /BBox [0 0 595 842]")},
		{6, "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>"},
	})
	text, err := ParsePDF(pdf)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if !strings.Contains(text, "Text in page") || !strings.Contains(text, "Text in form") {
		t.Errorf("表单 XObject 文本缺失: %q", text)
	}
}

// 页序按页树 Kids 顺序，而不是对象号顺序。
func TestParsePDFPageOrder(t *testing.T) {
	page := func(s string) string {
		return fmt.Sprintf("BT /F1 12 Tf 72 700 Td (%s) Tj ET", s)
	}
	c1, c2 := page("first page"), page("second page")
	pdf := buildPDF([]pdfObjSpec{
		{1, "<< /Type /Catalog /Pages 2 0 R >>"},
		{2, "<< /Type /Pages /Kids [4 0 R 3 0 R] /Count 2 >>"},
		{3, "<< /Type /Page /Parent 2 0 R /Resources << /Font << /F1 7 0 R >> >> /Contents 5 0 R >>"},
		{4, "<< /Type /Page /Parent 2 0 R /Resources << /Font << /F1 7 0 R >> >> /Contents 6 0 R >>"},
		{5, streamObj(c1)},
		{6, streamObj(c2)},
		{7, "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>"},
	})
	text, err := ParsePDF(pdf)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	// Kids 写的是 [4 0 R 3 0 R]，对象 4 的内容是 "second page" —— 页序必须跟着 Kids 走
	if strings.Index(text, "second page") > strings.Index(text, "first page") {
		t.Errorf("页序不对: %q", text)
	}
}

// 只有图片的 PDF（扫描件）要给出明确错误，而不是返回空字符串。
func TestParsePDFScannedImageOnly(t *testing.T) {
	img := []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 0x4a, 0x46}
	imgObj := fmt.Sprintf("<< /Type /XObject /Subtype /Image /Width 100 /Height 100 /Length %d /Filter /DCTDecode >>\nstream\n%s\nendstream", len(img), string(img))
	page := "q 595 0 0 842 0 0 cm /IMG Do Q\n"
	pdf := buildPDF([]pdfObjSpec{
		{1, "<< /Type /Catalog /Pages 2 0 R >>"},
		{2, "<< /Type /Pages /Kids [3 0 R] /Count 1 >>"},
		{3, "<< /Type /Page /Parent 2 0 R /Resources << /XObject << /IMG 5 0 R >> >> /Contents 4 0 R >>"},
		{4, streamObj(page)},
		{5, imgObj},
	})
	if _, err := ParsePDF(pdf); !errors.Is(err, ErrPDFNoTextLayer) {
		t.Fatalf("扫描件应返回 ErrPDFNoTextLayer，实际 %v", err)
	}
}

// AES-256（R5/R6）加密要明确报"不支持"，而不是含糊的失败。
func TestParsePDFAES256Encrypted(t *testing.T) {
	pdf := buildPDF([]pdfObjSpec{
		{1, "<< /Type /Catalog /Pages 2 0 R >>"},
		{2, "<< /Type /Pages /Kids [] /Count 0 >>"},
		{9, "<< /Filter /Standard /V 5 /R 6 /Length 256 /O <00> /U <00> /P -4 >>"},
	})
	pdf = bytes.Replace(pdf, []byte("/Root 1 0 R >>"), []byte("/Root 1 0 R /Encrypt 9 0 R >>"), 1)
	_, err := ParsePDF(pdf)
	if err == nil || !strings.Contains(err.Error(), "AES-256") {
		t.Fatalf("AES-256 应给出明确提示，实际 %v", err)
	}
}

// 只声明了 /Encrypt 但没有真实加密字典（对象不存在）时，不能假装加密，也不能崩。
func TestParsePDFBogusEncryptReference(t *testing.T) {
	pdf := buildPDF([]pdfObjSpec{
		{1, "<< /Type /Catalog /Pages 2 0 R >>"},
		{2, "<< /Type /Pages /Kids [3 0 R] /Count 1 >>"},
		{3, "<< /Type /Page /Parent 2 0 R /Resources << /Font << /F1 5 0 R >> >> /Contents 4 0 R >>"},
		{4, streamObj("BT /F1 12 Tf 72 700 Td (Still readable) Tj ET")},
		{5, "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>"},
	})
	pdf = bytes.Replace(pdf, []byte("/Root 1 0 R >>"), []byte("/Root 1 0 R /Encrypt 77 0 R >>"), 1)
	text, err := ParsePDF(pdf)
	if err != nil {
		t.Fatalf("应正常解析（/Encrypt 指向不存在的对象），实际 %v", err)
	}
	if !strings.Contains(text, "Still readable") {
		t.Errorf("文本缺失: %q", text)
	}
}

// 不是 PDF 的字节流要拒绝。
func TestParsePDFNotPDF(t *testing.T) {
	if _, err := ParsePDF([]byte("这不是 PDF")); !errors.Is(err, ErrUnsupportedFormat) {
		t.Fatalf("非 PDF 输入应返回 ErrUnsupportedFormat，实际 %v", err)
	}
}

// 走 ParseByFilename 的完整链路（导入接口用的就是它）。
func TestParseByFilenamePDF(t *testing.T) {
	text, encoding, err := ParseByFilename("book.PDF", simpleFontPDF())
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if !strings.Contains(text, "Hello PDF World") {
		t.Errorf("文本不对: %q", text)
	}
	if encoding != "PDF(文本层)" {
		t.Errorf("编码标记不对: %q", encoding)
	}
	if chapters := SplitChapters(text); len(chapters) == 0 {
		t.Error("PDF 文本切章失败")
	}
}
