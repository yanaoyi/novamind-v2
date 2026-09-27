package parser

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// ParseDOCX 从 .docx 字节流中抽取纯文本。
// 实现方式：解开 zip 读 word/document.xml，按 <w:p> 分段、取 <w:t> 文本。
// 不引入第三方依赖的原因：docx 正文抽取只需 XML 词法扫描，够用且可控。
func ParseDOCX(data []byte) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("docx 不是合法的 zip 包: %w", err)
	}

	var doc *zip.File
	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			doc = f
			break
		}
	}
	if doc == nil {
		return "", fmt.Errorf("docx 缺少 word/document.xml（可能是 .doc 旧格式或已损坏）")
	}

	rc, err := doc.Open()
	if err != nil {
		return "", fmt.Errorf("打开 document.xml 失败: %w", err)
	}
	defer rc.Close()

	text, err := extractDocumentText(rc)
	if err != nil {
		return "", err
	}
	return trimAll(text), nil
}

func extractDocumentText(r io.Reader) (string, error) {
	decoder := xml.NewDecoder(r)
	decoder.Strict = false // docx 里偶有未声明实体，宽松处理

	var sb strings.Builder
	inText := false

	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("解析 document.xml 失败: %w", err)
		}

		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "p":
				if sb.Len() > 0 {
					sb.WriteString("\n")
				}
			case "t":
				inText = true
			case "br", "cr":
				sb.WriteString("\n")
			case "tab":
				sb.WriteString("\t")
			}
		case xml.EndElement:
			if t.Name.Local == "t" {
				inText = false
			}
		case xml.CharData:
			if inText {
				sb.WriteString(string(t))
			}
		}
	}
	return sb.String(), nil
}
