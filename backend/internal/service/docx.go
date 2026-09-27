package service

import (
	"archive/zip"
	"bytes"
	"fmt"
	"strings"

	"github.com/yanaoyi/novamindv2/backend/internal/domain"
)

// buildDocx 生成一个最小可用的 .docx（Word 能打开）。
//
// 不引第三方库的原因：docx 本质是一个 zip + 三个 XML，导出正文只需要段落与文本，
// 自己拼比拉一个几十 MB 的依赖更可控（也避免中文模板兼容问题）。
func buildDocx(title string, chapters []domain.CreativeChapter) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	write := func(name, content string) error {
		w, err := zw.Create(name)
		if err != nil {
			return err
		}
		_, err = w.Write([]byte(content))
		return err
	}

	contentTypes := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
</Types>`
	rels := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`

	var doc strings.Builder
	doc.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>`)
	doc.WriteString(paragraph(title, true))
	for _, c := range chapters {
		doc.WriteString(paragraph(fmt.Sprintf("第 %d 章 %s", c.ChapterNo, c.Title), true))
		for _, line := range strings.Split(c.Content, "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			doc.WriteString(paragraph(line, false))
		}
	}
	doc.WriteString(`</w:body></w:document>`)

	if err := write("[Content_Types].xml", contentTypes); err != nil {
		return nil, fmt.Errorf("写入 docx 内容类型失败: %w", err)
	}
	if err := write("_rels/.rels", rels); err != nil {
		return nil, fmt.Errorf("写入 docx 关系失败: %w", err)
	}
	if err := write("word/document.xml", doc.String()); err != nil {
		return nil, fmt.Errorf("写入 docx 正文失败: %w", err)
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("打包 docx 失败: %w", err)
	}
	return buf.Bytes(), nil
}

func paragraph(text string, bold bool) string {
	escaped := escapeXML(text)
	if bold {
		return fmt.Sprintf(`<w:p><w:r><w:rPr><w:b/></w:rPr><w:t xml:space="preserve">%s</w:t></w:r></w:p>`, escaped)
	}
	return fmt.Sprintf(`<w:p><w:r><w:t xml:space="preserve">%s</w:t></w:r></w:p>`, escaped)
}

func escapeXML(s string) string {
	replacer := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&apos;")
	return replacer.Replace(s)
}
