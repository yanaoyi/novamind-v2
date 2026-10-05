package parser

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
)

// EPUB / MOBI 解析测试。
//
// 合成样本的价值：不依赖任何外部文件就能守住边界（spine 顺序、实体、script 丢弃、
// PalmDOC 的三条规则、未压缩、HUFF 明确报错）。真实书库的对照验证另见
// scripts/check-book-extract.sh（用同一本书的 epub 与 mobi 互相当基准）。

func buildEPUB(t *testing.T, files map[string]string, spine []string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("写入 zip 条目失败: %v", err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatalf("写入 zip 内容失败: %v", err)
		}
	}
	_ = spine
	if err := zw.Close(); err != nil {
		t.Fatalf("关闭 zip 失败: %v", err)
	}
	return buf.Bytes()
}

func TestParseEPUBFollowsSpineOrderAndStripsTags(t *testing.T) {
	files := map[string]string{
		"META-INF/container.xml": `<?xml version="1.0"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles>
</container>`,
		"OEBPS/content.opf": `<?xml version="1.0"?>
<package xmlns="http://www.idpf.org/2007/opf">
  <manifest>
    <item id="c2" href="text/ch2.xhtml" media-type="application/xhtml+xml"/>
    <item id="c1" href="text/ch1.xhtml" media-type="application/xhtml+xml"/>
    <item id="css" href="style.css" media-type="text/css"/>
  </manifest>
  <spine><itemref idref="c1"/><itemref idref="c2"/></spine>
</package>`,
		"OEBPS/text/ch1.xhtml": `<html><head><title>别进来</title><style>p{color:red}</style></head>
<body><h1>第一章 雨夜</h1><p>沈砚推开老宅的门&amp;取出青铜钥匙。</p>
<script>alert('不该出现')</script><p>左肩的箭伤未愈&nbsp;他仍握紧了刀。</p></body></html>`,
		"OEBPS/text/ch2.xhtml": `<html><body><h1>第二章 夹墙</h1><p>夹墙里是三十年前的军需账册。</p></body></html>`,
		"OEBPS/style.css":      `body { font-family: serif; }`,
	}
	text, err := ParseEPUB(buildEPUB(t, files, nil))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	// spine 顺序：ch1 在前
	if i1, i2 := strings.Index(text, "第一章"), strings.Index(text, "第二章"); i1 < 0 || i2 < 0 || i1 > i2 {
		t.Fatalf("应按 spine 顺序输出，实际:\n%s", text)
	}
	for _, want := range []string{"沈砚推开老宅的门&取出青铜钥匙", "左肩的箭伤未愈 他仍握紧了刀"} {
		if !strings.Contains(text, want) {
			t.Errorf("正文应含 %q，实际:\n%s", want, text)
		}
	}
	for _, bad := range []string{"别进来", "alert", "font-family"} {
		if strings.Contains(text, bad) {
			t.Errorf("script/style/title 的内容不该进正文，却出现了 %q:\n%s", bad, text)
		}
	}
}

func TestParseEPUBRejectsNonZipAndMissingContainer(t *testing.T) {
	if _, err := ParseEPUB([]byte("这不是 zip")); err == nil {
		t.Error("非 zip 应报错")
	}
	files := map[string]string{"OEBPS/content.opf": `<package><manifest/></package>`}
	if _, err := ParseEPUB(buildEPUB(t, files, nil)); err == nil {
		t.Error("缺 container.xml 应报错（否则会误判成可读的电子书）")
	}
}

// ---------- MOBI ----------

// palmdocCompressLiterals 生成"全字面量"的 PalmDOC 流（用于合成测试样本）。
// 规则对应解码：0x09-0x7F 直接写；其余字节用 0x01-0x08 批量原样复制。
func palmdocCompressLiterals(text []byte) []byte {
	var out []byte
	i := 0
	needsEscape := func(b byte) bool { return b < 0x09 || b > 0x7F }
	for i < len(text) {
		// 连续可直写的字面量
		if !needsEscape(text[i]) {
			out = append(out, text[i])
			i++
			continue
		}
		// 需要转义的字节：用 0x01-0x08 原样搬运（每段最多 8 个）
		n := 0
		for i+n < len(text) && n < 8 && needsEscape(text[i+n]) {
			n++
		}
		if n == 0 {
			out = append(out, text[i])
			i++
			continue
		}
		out = append(out, byte(n))
		out = append(out, text[i:i+n]...)
		i += n
	}
	return out
}

func buildMOBI(t *testing.T, text []byte, compression uint16) []byte {
	t.Helper()
	const recordSize = 4096
	var records [][]byte
	switch compression {
	case 1:
		for i := 0; i < len(text); i += recordSize {
			end := i + recordSize
			if end > len(text) {
				end = len(text)
			}
			records = append(records, append([]byte{}, text[i:end]...))
		}
	case 2:
		for i := 0; i < len(text); i += recordSize {
			end := i + recordSize
			if end > len(text) {
				end = len(text)
			}
			records = append(records, palmdocCompressLiterals(text[i:end]))
		}
	default:
		t.Fatalf("测试只合成 compression=1/2，收到 %d", compression)
	}

	// 记录 0：PalmDOC 头（16B）+ MOBI 头
	rec0 := make([]byte, 16+232)
	binary.BigEndian.PutUint16(rec0[0:2], compression)
	binary.BigEndian.PutUint32(rec0[4:8], uint32(len(text)))
	binary.BigEndian.PutUint16(rec0[8:10], uint16(len(records)))
	binary.BigEndian.PutUint16(rec0[10:12], recordSize)
	copy(rec0[16:20], "MOBI")
	binary.BigEndian.PutUint32(rec0[20:24], 232)
	binary.BigEndian.PutUint32(rec0[28:32], mobiTextEncUTF8)
	binary.BigEndian.PutUint32(rec0[36:40], 6)

	all := append([][]byte{rec0}, records...)
	header := make([]byte, pdbHeaderSize)
	copy(header[0:32], "TestBook")
	copy(header[60:64], "BOOK")
	copy(header[64:68], "MOBI")
	binary.BigEndian.PutUint16(header[76:78], uint16(len(all)))

	offset := pdbHeaderSize + len(all)*pdbRecordEntry
	index := make([]byte, len(all)*pdbRecordEntry)
	var body bytes.Buffer
	for i, rec := range all {
		binary.BigEndian.PutUint32(index[i*8:i*8+4], uint32(offset))
		index[i*8+4] = 0
		offset += len(rec)
		body.Write(rec)
	}
	out := append(header, index...)
	return append(out, body.Bytes()...)
}

func TestParseMOBIUncompressedAndPalmDOC(t *testing.T) {
	long := strings.Repeat("<p>沈砚推开老宅的门，取出青铜钥匙。</p>\n", 400) // 超一条记录
	for _, compression := range []uint16{1, 2} {
		data := buildMOBI(t, []byte(long), compression)
		text, err := ParseMOBI(data)
		if err != nil {
			t.Fatalf("compression=%d 解析失败: %v", compression, err)
		}
		if n := strings.Count(text, "青铜钥匙"); n != 400 {
			t.Errorf("compression=%d 应完整解出 400 处正文，实际 %d", compression, n)
		}
		if strings.Contains(text, "<p>") {
			t.Errorf("compression=%d 应剥掉 HTML 标签", compression)
		}
	}
}

// PalmDOC 的"长度-距离"回指必须真的被解出来（否则长文本会缺字）。
func TestPalmDOCResolvesBackReferences(t *testing.T) {
	// "abcdef" 之后用一对 (距离=6, 长度=6) 回指，得到 "abcdefabcdef"
	var src []byte
	src = append(src, []byte("abcdef")...)
	pair := (6 << 3) | (6 - 3) // distance=6, length=6
	src = append(src, byte(0x80|(pair>>8)), byte(pair&0xFF))
	out, err := palmdocDecompress(src, 12)
	if err != nil {
		t.Fatalf("解压失败: %v", err)
	}
	if string(out) != "abcdefabcdef" {
		t.Errorf("回指应解出 abcdefabcdef，实际 %q", string(out))
	}
}

func TestParseMOBIReportsHUFFClearly(t *testing.T) {
	data := buildMOBI(t, []byte("正文"), 1)
	// 把记录 0 的压缩字段改成 HUFF/CDIC
	rec0 := int(binary.BigEndian.Uint32(data[78:82]))
	binary.BigEndian.PutUint16(data[rec0:rec0+2], 17480)
	_, err := ParseMOBI(data)
	if err == nil || !strings.Contains(err.Error(), "HUFF/CDIC") {
		t.Errorf("HUFF/CDIC 应给出明确的、可操作的错误，实际 %v", err)
	}
}

func TestStripHTMLToTextHandlesEdgeCases(t *testing.T) {
	cases := []struct{ in, want string }{
		{`<p>a<br/>b</p>`, "a\nb"},
		{`<div>第一段</div><div>第二段</div>`, "第一段\n第二段"},
		{`a &lt;b&gt; c &amp; d`, "a <b> c & d"},
		{`&#31532;&#19968;章`, "第一章"},
		{`<span title="a>b">文字</span>`, "文字"},
		{`<!-- 注释 -->正文<![CDATA[原样<保留>]]>`, "正文原样<保留>"},
		{`<table><tr><td>甲</td><td>乙</td></tr></table>`, "甲 乙"},
	}
	for _, c := range cases {
		if got := StripHTMLToText(c.in); got != c.want {
			t.Errorf("StripHTMLToText(%q) = %q，期望 %q", c.in, got, c.want)
		}
	}
}
