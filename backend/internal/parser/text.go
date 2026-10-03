// Package parser 负责把上传文档转成纯文本，并做原著章节切分。
//
// 设计要点：
//   - 编码：中文 TXT 大量是 GBK/GB18030，必须探测后再解码，否则整本书都是乱码；
//   - 章节：按中文网文常见标题形式识别，并记录每章在全文中的字节偏移（规格书 §9）；
//   - 兜底：识别不到标题时按长度切分，保证"导入即可用"，而不是整本变成一章。
package parser

import (
	"bytes"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

// Chapter 是切分结果。
type Chapter struct {
	No      int
	Title   string
	Content string
	// Start/End 是在解码后全文中的 UTF-8 字节偏移（[Start, End)），用于回溯原文
	Start int64
	End   int64
}

// DefaultChunkRunes 是识别不到章节标题时的兜底切分粒度。
const DefaultChunkRunes = 8000

// 章节标题候选（行首、整行短句）。
var chapterPatterns = []*regexp.Regexp{
	// 第一章 / 第 12 章 / 第一百二十三回 / 第三卷
	regexp.MustCompile(`^第\s*[0-9〇零一二三四五六七八九十百千万两]{1,12}\s*[章回节卷篇][^\n]{0,60}$`),
	// Chapter 12 / CHAPTER 12: xxx
	regexp.MustCompile(`^(?i:chapter)\s*[0-9]{1,5}\s*[:：.、]?[^\n]{0,60}$`),
	// 序章 / 楔子 / 尾声 / 番外 等
	regexp.MustCompile(`^(序章|楔子|引子|前言|序言|尾声|后记|番外)[^\n]{0,40}$`),
}

// ErrEmptyText 表示解码后没有内容。
var ErrEmptyText = errors.New("文件中没有可解析的文本")

// DecodeText 探测编码并解码为 UTF-8，返回文本与实际使用的编码名。
func DecodeText(data []byte) (string, string, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return "", "", ErrEmptyText
	}

	// 1) BOM 显式声明
	switch {
	case bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF}):
		return trimAll(string(data[3:])), "UTF-8(BOM)", nil
	case bytes.HasPrefix(data, []byte{0xFF, 0xFE}):
		return decodeWith(data[2:], unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM).NewDecoder(), "UTF-16LE")
	case bytes.HasPrefix(data, []byte{0xFE, 0xFF}):
		return decodeWith(data[2:], unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM).NewDecoder(), "UTF-16BE")
	}

	// 2) 已是合法 UTF-8 直接用
	if utf8.Valid(data) {
		return trimAll(string(data)), "UTF-8", nil
	}

	// 3) 候选编码逐个尝试，选"乱码最少"的
	candidates := []struct {
		name string
		dec  transform.Transformer
	}{
		{"GB18030", simplifiedchinese.GB18030.NewDecoder()},
		{"Big5", traditionalchinese.Big5.NewDecoder()},
	}
	bestName, bestText, bestScore := "", "", -1.0
	for _, c := range candidates {
		text, err := transformBytes(data, c.dec)
		if err != nil {
			continue
		}
		if score := textScore(text); score > bestScore {
			bestName, bestText, bestScore = c.name, text, score
		}
	}
	if bestScore < 0 {
		return "", "", errors.New("无法识别文件编码")
	}
	return trimAll(bestText), bestName, nil
}

// SplitChapters 把全文切成章节：优先按标题切，无标题则按长度兜底。
func SplitChapters(text string) []Chapter {
	lines := scanLines(text)

	type mark struct {
		lineIdx int
		title   string
	}
	var marks []mark
	for i, ln := range lines {
		if title, ok := matchChapterTitle(ln.text); ok {
			marks = append(marks, mark{lineIdx: i, title: title})
		}
	}

	if len(marks) == 0 {
		return chunkByLength(text, DefaultChunkRunes)
	}

	var chapters []Chapter
	// 第一个标题之前的实质内容作为"开篇"保留，避免丢字
	if headEnd := lines[marks[0].lineIdx].start; strings.TrimSpace(text[:headEnd]) != "" {
		chapters = append(chapters, Chapter{
			No:      1,
			Title:   "开篇",
			Content: strings.TrimSpace(text[:headEnd]),
			Start:   0,
			End:     int64(headEnd),
		})
	}

	for idx, m := range marks {
		start := lines[m.lineIdx].start
		end := len(text)
		if idx+1 < len(marks) {
			end = lines[marks[idx+1].lineIdx].start
		}
		chapters = append(chapters, Chapter{
			No:      len(chapters) + 1,
			Title:   m.title,
			Content: strings.TrimSpace(text[start:end]),
			Start:   int64(start),
			End:     int64(end),
		})
	}
	return chapters
}

type textLine struct {
	text  string
	start int
	end   int
}

// scanLines 按行扫描并记录每行的字节区间。
func scanLines(text string) []textLine {
	var lines []textLine
	start := 0
	for i := 0; i < len(text); i++ {
		if text[i] == '\n' {
			lines = append(lines, textLine{text: text[start:i], start: start, end: i + 1})
			start = i + 1
		}
	}
	if start < len(text) {
		lines = append(lines, textLine{text: text[start:], start: start, end: len(text)})
	}
	return lines
}

// matchChapterTitle 判断一行是否为章节标题，返回规范化标题。
func matchChapterTitle(raw string) (string, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", false
	}
	// 标题行不会太长，避免把正文里提到"第一章"的整段当成标题
	if utf8.RuneCountInString(trimmed) > 60 {
		return "", false
	}
	for _, p := range chapterPatterns {
		if p.MatchString(trimmed) {
			return strings.Join(strings.Fields(trimmed), " "), true
		}
	}
	return "", false
}

// chunkByLength 按 rune 数兜底切分。
func chunkByLength(text string, maxRunes int) []Chapter {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) == 0 {
		return nil
	}
	var chapters []Chapter
	for start := 0; start < len(runes); start += maxRunes {
		end := start + maxRunes
		if end > len(runes) {
			end = len(runes)
		}
		no := len(chapters) + 1
		chapters = append(chapters, Chapter{
			No:      no,
			Title:   "第 " + strconv.Itoa(no) + " 段（自动切分）",
			Content: strings.TrimSpace(string(runes[start:end])),
			Start:   int64(len(string(runes[:start]))),
			End:     int64(len(string(runes[:end]))),
		})
	}
	return chapters
}

func decodeWith(data []byte, t transform.Transformer, name string) (string, string, error) {
	text, err := transformBytes(data, t)
	if err != nil {
		return "", "", err
	}
	return trimAll(text), name, nil
}

func transformBytes(data []byte, t transform.Transformer) (string, error) {
	out, _, err := transform.Bytes(t, data)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// textScore 给解码结果打分：可读字符（CJK/ASCII/标点）占比，乱码重罚。
func textScore(text string) float64 {
	if text == "" {
		return 0
	}
	good, total := 0, 0
	for _, r := range text {
		total++
		switch {
		case r == utf8.RuneError:
			good -= 5
		case r == '\n' || r == '\r' || r == '\t':
			good++
		case r >= 0x20 && r < 0x7F:
			good++
		case r >= 0x4E00 && r <= 0x9FFF: // 常用汉字
			good += 2
		case r >= 0x3000 && r <= 0x303F: // 中文标点
			good++
		case r >= 0xFF00 && r <= 0xFFEF: // 全角字符
			good++
		}
	}
	return float64(good) / float64(total)
}

func trimAll(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = strings.TrimPrefix(s, "\uFEFF")
	return strings.TrimSpace(s)
}
