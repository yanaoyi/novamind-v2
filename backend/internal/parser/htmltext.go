package parser

import (
	"strings"
)

// (X)HTML → 纯文本（EPUB 与 MOBI 共用）。
//
// 为什么手写而不引第三方 HTML 解析库：
//   - 我们只要"文本 + 段落换行"，不需要 DOM 树；
//   - 电子书里的 HTML 常常不完全规范（未闭合标签、裸 & 号、CDATA），
//     严格的解析器遇到就整篇报错，而这里更希望"尽量抽出能读的正文"；
//   - 与 DOCX/PDF 解析器一样保持零依赖。
//
// 行为：块级标签与 <br> 变成换行；script/style 内容丢弃；注释/CDATA 正确处理；
// 常见命名实体还原；连续空白折叠成单个空格；空行折叠。

// blockTags 是"前后要断开成行"的标签。
var blockTags = map[string]bool{
	"p": true, "div": true, "br": true, "hr": true, "tr": true, "li": true,
	"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
	"blockquote": true, "section": true, "article": true, "pre": true,
	"figcaption": true, "table": true, "dt": true, "dd": true, "center": true,
}

// skipTags 是"整段内容都要丢掉"的标签（样式与脚本不是正文）。
var skipTags = map[string]bool{"script": true, "style": true, "head": true, "title": true}

// htmlEntities 是 XML 之外的常见命名实体（XML 自带的 amp/lt/gt/quot/apos 由解码路径统一处理）。
var htmlEntities = map[string]string{
	"nbsp": "\u00a0", "mdash": "—", "ndash": "–", "hellip": "…",
	"ldquo": "“", "rdquo": "”", "lsquo": "‘", "rsquo": "’",
	"middot": "·", "bull": "•", "times": "×", "divide": "÷",
	"laquo": "«", "raquo": "»", "copy": "©", "reg": "®", "trade": "™",
	"deg": "°", "sect": "§", "para": "¶", "dagger": "†",
	"euro": "€", "pound": "£", "yen": "¥", "cent": "¢",
	"alpha": "α", "beta": "β", "gamma": "γ", "delta": "δ",
	"Delta": "Δ", "Omega": "Ω", "pi": "π", "sigma": "σ",
}

// StripHTMLToText 把 HTML/XHTML 片段转成纯文本。
func StripHTMLToText(s string) string {
	// 既没有标签也没有实体时才走快速路径 —— 少了 '&' 的判断，
	// "a &lt;b&gt;" 这种纯实体文本会被原样返回（单测抓到过）。
	if !strings.ContainsRune(s, '<') && !strings.ContainsRune(s, '&') {
		return cleanText(s)
	}
	var sb strings.Builder
	i := 0
	n := len(s)
	skipDepthTag := "" // 非空表示正在丢弃某个标签的内容

	writeText := func(raw string) {
		if skipDepthTag != "" {
			return
		}
		text := unescapeEntities(raw)
		text = collapseSpaces(text)
		if text == "" {
			return
		}
		// 前一个字符是换行/行首时不要前导空格
		if cur := sb.String(); cur != "" {
			last := cur[len(cur)-1]
			if text[0] == ' ' && (last == '\n' || last == ' ') {
				text = text[1:]
			}
			if text == "" {
				return
			}
		} else {
			text = strings.TrimLeft(text, " ")
		}
		sb.WriteString(text)
	}
	breakLine := func() {
		if skipDepthTag != "" {
			return
		}
		cur := sb.String()
		if cur == "" || strings.HasSuffix(cur, "\n") {
			return
		}
		// 去掉行尾空格再换行
		trimmed := strings.TrimRight(cur, " ")
		sb.Reset()
		sb.WriteString(trimmed)
		sb.WriteByte('\n')
	}

	for i < n {
		if s[i] != '<' {
			j := strings.IndexByte(s[i:], '<')
			if j < 0 {
				j = n - i
			}
			writeText(s[i : i+j])
			i += j
			continue
		}
		switch {
		case strings.HasPrefix(s[i:], "<!--"):
			end := strings.Index(s[i:], "-->")
			if end < 0 {
				i = n
			} else {
				i += end + 3
			}
			continue
		case strings.HasPrefix(s[i:], "<![CDATA["):
			end := strings.Index(s[i:], "]]>")
			if end < 0 {
				i = n
			} else {
				writeText(s[i+9 : i+end])
				i += end + 3
			}
			continue
		case strings.HasPrefix(s[i:], "<!"), strings.HasPrefix(s[i:], "<?"):
			// DOCTYPE / 处理指令：整段跳过
			end := tagEnd(s, i)
			if end < 0 {
				i = n
			} else {
				i = end + 1
			}
			continue
		}

		end := tagEnd(s, i)
		if end < 0 {
			// 没有闭合的 '<'：按普通文本处理，避免丢内容
			writeText(s[i:])
			break
		}
		tag := s[i+1 : end]
		i = end + 1

		name, closing, selfClosing := parseTag(tag)
		if name == "" {
			continue
		}
		if skipDepthTag != "" {
			if closing && name == skipDepthTag {
				skipDepthTag = ""
			}
			continue
		}
		if skipTags[name] {
			if !closing && !selfClosing {
				skipDepthTag = name
			}
			continue
		}
		if blockTags[name] {
			breakLine()
			continue
		}
		if name == "td" || name == "th" {
			writeText(" ")
		}
	}
	return cleanText(sb.String())
}

// tagEnd 找标签结束的 '>'，跳过引号内的 '>'（属性值里出现 '>' 是合法的）。
func tagEnd(s string, start int) int {
	quote := byte(0)
	for i := start + 1; i < len(s); i++ {
		c := s[i]
		if quote != 0 {
			if c == quote {
				quote = 0
			}
			continue
		}
		switch c {
		case '"', '\'':
			quote = c
		case '>':
			return i
		}
	}
	return -1
}

// parseTag 解析标签名与是否闭合/自闭合。
func parseTag(tag string) (name string, closing, selfClosing bool) {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return "", false, false
	}
	if tag[0] == '/' {
		closing = true
		tag = strings.TrimSpace(tag[1:])
	}
	if strings.HasSuffix(tag, "/") {
		selfClosing = true
		tag = strings.TrimSpace(strings.TrimSuffix(tag, "/"))
	}
	end := 0
	for end < len(tag) {
		c := tag[end]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '/' {
			break
		}
		end++
	}
	if end == 0 {
		return "", closing, selfClosing
	}
	return strings.ToLower(tag[:end]), closing, selfClosing
}

// unescapeEntities 还原命名实体与数字实体。
//
// 与"先替换 &amp; 再解析"的常见做法不同：这里一次性扫描，不会出现
// "&amp;lt;" 被二次还原成 "<" 这类问题。
func unescapeEntities(s string) string {
	if !strings.ContainsRune(s, '&') {
		return s
	}
	var sb strings.Builder
	sb.Grow(len(s))
	for i := 0; i < len(s); {
		if s[i] != '&' {
			sb.WriteByte(s[i])
			i++
			continue
		}
		semi := strings.IndexByte(s[i:], ';')
		if semi < 0 || semi > 12 {
			sb.WriteByte(s[i])
			i++
			continue
		}
		body := s[i+1 : i+semi]
		if r, ok := entityRune(body); ok {
			sb.WriteRune(r)
			i += semi + 1
			continue
		}
		sb.WriteByte(s[i])
		i++
	}
	return sb.String()
}

func entityRune(body string) (rune, bool) {
	switch body {
	case "amp":
		return '&', true
	case "lt":
		return '<', true
	case "gt":
		return '>', true
	case "quot":
		return '"', true
	case "apos":
		return '\'', true
	}
	if strings.HasPrefix(body, "#") {
		digits := body[1:]
		base := 10
		if len(digits) > 0 && (digits[0] == 'x' || digits[0] == 'X') {
			base = 16
			digits = digits[1:]
		}
		if digits == "" || len(digits) > 8 {
			return 0, false
		}
		var value int64
		for _, c := range digits {
			d := digitValue(c, base)
			if d < 0 {
				return 0, false
			}
			value = value*int64(base) + int64(d)
			if value > 0x10FFFF {
				return 0, false
			}
		}
		return rune(value), true
	}
	if v, ok := htmlEntities[body]; ok && v != "" {
		return []rune(v)[0], true
	}
	return 0, false
}

func digitValue(c rune, base int) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case base == 16 && c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case base == 16 && c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	default:
		return -1
	}
}

// collapseSpaces 把制表/换行/连续空格折叠成单个空格（HTML 的空白语义）。
func collapseSpaces(s string) string {
	var sb strings.Builder
	sb.Grow(len(s))
	lastSpace := false
	for _, r := range s {
		switch r {
		case ' ', '\t', '\n', '\r', '\f', '\v', '\u00a0':
			if !lastSpace {
				sb.WriteByte(' ')
				lastSpace = true
			}
		default:
			sb.WriteRune(r)
			lastSpace = false
		}
	}
	return sb.String()
}

// cleanText 收尾：统一换行、去掉行尾空白、把连续空行压成一个、去掉首尾空行。
//
// 为什么不能直接 trimAll：HTML 剥完标签会留下大量空行（每个块级标签都换行），
// 不压缩的话一章正文里会夹几百个空行，切章与字数统计都会受影响。
func cleanText(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	blank := false
	for _, line := range lines {
		line = strings.TrimRight(line, " \t\u00a0")
		if line == "" {
			if len(out) == 0 || blank {
				continue
			}
			blank = true
			out = append(out, "")
			continue
		}
		blank = false
		out = append(out, line)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}
