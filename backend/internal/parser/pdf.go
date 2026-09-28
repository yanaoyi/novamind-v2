package parser

// PDF 文本层抽取（自建实现，不依赖第三方库）。
//
// 为什么自己写：本机 Go 模块缓存里没有任何 PDF 库，依赖网络拉包在这台机器上不稳定，
// 而「把 PDF 里的文本抽出来」这件事在文本型 PDF 上是可以精确做到的：
//
//   1. 扫描文件里的 N G obj ... endobj，拿到对象表（不解析 xref，避免 xref 表/流两种形态的兼容负担）；
//   2. 顺带解压对象流（/Type /ObjStm），因为现代 PDF 会把大量对象压在里面；
//   3. 从 /Catalog → /Pages 递归取页面（按阅读顺序），并沿页树继承 /Resources；
//   4. 解 /Contents 内容流（FlateDecode 等 + PNG/TIFF 预测器），按算子抽文本；
//   5. 每个字体优先用 /ToUnicode CMap 把字符码映射成 Unicode —— 中文 PDF 基本都是 Type0(CID)，
//      没有这一步抽出来就是乱码；没有 ToUnicode 的简单字体退回 WinAnsi/Latin-1。
//
// 明确不做的：加密 PDF（报错提示先解密）、纯扫描件（报错提示需要 OCR）、
// 图片类滤镜（DCT/JPX/CCITT/JBIG2，遇到就跳过该流）。

import (
	"bytes"
	"compress/flate"
	"compress/zlib"
	"encoding/ascii85"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
	"golang.org/x/text/encoding/unicode"
)

// ErrPDFNoTextLayer 表示 PDF 里没有文本层（多半是扫描件）。
var ErrPDFNoTextLayer = errors.New("PDF 中没有可提取的文本层（可能是扫描件，需要先做 OCR）")

// ErrPDFEncrypted 表示 PDF 被加密，需要先解除保护。
var ErrPDFEncrypted = errors.New("PDF 已加密，无法解析（请先解除密码保护）")

const maxFormDepth = 6

// ---------- 对象模型 ----------

type (
	pdfKeyword string
	pdfName    string
	pdfString  []byte
	pdfArray   []any
	pdfDict    map[string]any
	pdfRef     int
	pdfStream  struct {
		dict pdfDict
		data []byte
	}
)

// ---------- 词法/语法 ----------

type pdfScanner struct {
	buf []byte
	pos int
}

func isPDFWhite(c byte) bool {
	return c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == '\f' || c == 0
}

func isPDFDelim(c byte) bool {
	switch c {
	case '(', ')', '<', '>', '[', ']', '{', '}', '/', '%':
		return true
	}
	return false
}

func (s *pdfScanner) skipSpace() {
	for s.pos < len(s.buf) {
		c := s.buf[s.pos]
		switch {
		case c == '%':
			for s.pos < len(s.buf) && s.buf[s.pos] != '\n' && s.buf[s.pos] != '\r' {
				s.pos++
			}
		case isPDFWhite(c):
			s.pos++
		default:
			return
		}
	}
}

// parseObject 解析一个 PDF 对象（含引用与关键字）。
func (s *pdfScanner) parseObject() (any, error) {
	s.skipSpace()
	if s.pos >= len(s.buf) {
		return nil, io.EOF
	}
	switch c := s.buf[s.pos]; {
	case c == '/':
		return s.parseName(), nil
	case c == '(':
		return s.parseLiteralString()
	case c == '<':
		if s.pos+1 < len(s.buf) && s.buf[s.pos+1] == '<' {
			return s.parseDict()
		}
		return s.parseHexString()
	case c == '[':
		return s.parseArray()
	case c == ']' || c == '>' || c == ')' || c == '}':
		s.pos++
		return s.parseObject()
	case c == '{':
		return s.parseDict()
	case c == '+' || c == '-' || c == '.' || (c >= '0' && c <= '9'):
		return s.parseNumberOrRef()
	default:
		return s.parseKeyword(), nil
	}
}

func (s *pdfScanner) parseName() pdfName {
	s.pos++ // '/'
	start := s.pos
	var sb strings.Builder
	for s.pos < len(s.buf) {
		c := s.buf[s.pos]
		if isPDFWhite(c) || isPDFDelim(c) {
			break
		}
		if c == '#' && s.pos+2 < len(s.buf) {
			if b, err := strconv.ParseUint(string(s.buf[s.pos+1:s.pos+3]), 16, 8); err == nil {
				sb.WriteString(string(s.buf[start:s.pos]))
				sb.WriteByte(byte(b))
				s.pos += 3
				start = s.pos
				continue
			}
		}
		s.pos++
	}
	sb.WriteString(string(s.buf[start:s.pos]))
	return pdfName(sb.String())
}

func (s *pdfScanner) parseLiteralString() (pdfString, error) {
	s.pos++ // '('
	depth := 1
	out := make([]byte, 0, 32)
	for s.pos < len(s.buf) {
		c := s.buf[s.pos]
		switch c {
		case '\\':
			s.pos++
			if s.pos >= len(s.buf) {
				return pdfString(out), nil
			}
			e := s.buf[s.pos]
			switch e {
			case 'n':
				out = append(out, '\n')
			case 'r':
				out = append(out, '\r')
			case 't':
				out = append(out, '\t')
			case 'b':
				out = append(out, '\b')
			case 'f':
				out = append(out, '\f')
			case '(', ')', '\\':
				out = append(out, e)
			case '\r':
				if s.pos+1 < len(s.buf) && s.buf[s.pos+1] == '\n' {
					s.pos++
				}
			case '\n':
			default:
				if e >= '0' && e <= '7' {
					val := 0
					for i := 0; i < 3 && s.pos < len(s.buf); i++ {
						d := s.buf[s.pos]
						if d < '0' || d > '7' {
							break
						}
						val = val*8 + int(d-'0')
						s.pos++
					}
					out = append(out, byte(val))
					continue
				}
				out = append(out, e)
			}
			s.pos++
		case '(':
			depth++
			out = append(out, c)
			s.pos++
		case ')':
			depth--
			s.pos++
			if depth == 0 {
				return pdfString(out), nil
			}
			out = append(out, c)
		default:
			out = append(out, c)
			s.pos++
		}
	}
	return pdfString(out), nil
}

func (s *pdfScanner) parseHexString() (pdfString, error) {
	s.pos++ // '<'
	digits := make([]byte, 0, 32)
	for s.pos < len(s.buf) && s.buf[s.pos] != '>' {
		c := s.buf[s.pos]
		s.pos++
		if isPDFWhite(c) {
			continue
		}
		digits = append(digits, c)
	}
	if s.pos < len(s.buf) {
		s.pos++ // '>'
	}
	if len(digits)%2 == 1 {
		digits = append(digits, '0')
	}
	out := make([]byte, 0, len(digits)/2)
	for i := 0; i+1 < len(digits); i += 2 {
		b, err := strconv.ParseUint(string(digits[i:i+2]), 16, 8)
		if err != nil {
			continue
		}
		out = append(out, byte(b))
	}
	return pdfString(out), nil
}

func (s *pdfScanner) parseArray() (pdfArray, error) {
	s.pos++ // '['
	out := pdfArray{}
	for {
		s.skipSpace()
		if s.pos >= len(s.buf) {
			return out, nil
		}
		if s.buf[s.pos] == ']' {
			s.pos++
			return out, nil
		}
		obj, err := s.parseObject()
		if err != nil {
			return out, nil
		}
		out = append(out, obj)
	}
}

func (s *pdfScanner) parseDict() (pdfDict, error) {
	if s.pos+1 < len(s.buf) && s.buf[s.pos] == '<' && s.buf[s.pos+1] == '<' {
		s.pos += 2
	} else {
		s.pos++
	}
	out := pdfDict{}
	for {
		s.skipSpace()
		if s.pos >= len(s.buf) {
			return out, nil
		}
		if s.buf[s.pos] == '>' {
			s.pos++
			if s.pos < len(s.buf) && s.buf[s.pos] == '>' {
				s.pos++
			}
			return out, nil
		}
		if s.buf[s.pos] == '}' {
			s.pos++
			return out, nil
		}
		if s.buf[s.pos] != '/' {
			if _, err := s.parseObject(); err != nil {
				return out, nil
			}
			continue
		}
		key := string(s.parseName())
		val, err := s.parseObject()
		if err != nil {
			return out, nil
		}
		out[key] = val
	}
}

func (s *pdfScanner) parseNumberOrRef() (any, error) {
	start := s.pos
	for s.pos < len(s.buf) {
		c := s.buf[s.pos]
		if (c >= '0' && c <= '9') || c == '+' || c == '-' || c == '.' {
			s.pos++
			continue
		}
		// 科学计数法（1e-3 这类在做表工具生成的 PDF 里真的会出现）
		if (c == 'e' || c == 'E') && s.pos+1 < len(s.buf) {
			next := s.buf[s.pos+1]
			if (next >= '0' && next <= '9') || ((next == '+' || next == '-') && s.pos+2 < len(s.buf)) {
				s.pos++
				continue
			}
			if s.pos+2 < len(s.buf) && s.buf[s.pos+2] >= '0' && s.buf[s.pos+2] <= '9' {
				s.pos++
				continue
			}
		}
		break
	}
	raw := string(s.buf[start:s.pos])
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0.0, nil
	}
	if value == math.Trunc(value) && value >= 0 && value < 1e9 {
		save := s.pos
		s.skipSpace()
		genStart := s.pos
		for s.pos < len(s.buf) && s.buf[s.pos] >= '0' && s.buf[s.pos] <= '9' {
			s.pos++
		}
		if s.pos > genStart {
			if _, err := strconv.Atoi(string(s.buf[genStart:s.pos])); err == nil {
				s.skipSpace()
				if s.pos < len(s.buf) && s.buf[s.pos] == 'R' &&
					(s.pos+1 >= len(s.buf) || isPDFWhite(s.buf[s.pos+1]) || isPDFDelim(s.buf[s.pos+1])) {
					s.pos++
					return pdfRef(int(value)), nil
				}
			}
		}
		s.pos = save
	}
	return value, nil
}

func (s *pdfScanner) parseKeyword() pdfKeyword {
	start := s.pos
	for s.pos < len(s.buf) {
		c := s.buf[s.pos]
		if isPDFWhite(c) || isPDFDelim(c) {
			break
		}
		s.pos++
	}
	if s.pos == start {
		s.pos++
	}
	return pdfKeyword(s.buf[start:s.pos])
}

// ---------- 文档 ----------

type pdfDoc struct {
	objs  map[int]any
	order []int
	// fontCache：按字体对象号缓存（含内嵌字体 cmap 反查表）。
	// 不加这个，200 页的报表会把同一个 870KB 的内嵌字体解析上千遍（实测跑一次要两分钟）。
	fontCache map[int]*pdfFont
}

// ParsePDF 从 PDF 字节里抽出正文（UTF-8）。
func ParsePDF(data []byte) (string, error) {
	trimmed := bytes.TrimLeft(data, " \t\r\n\f\x00")
	if !bytes.HasPrefix(trimmed, []byte("%PDF-")) {
		return "", fmt.Errorf("%w：文件头不是 %%PDF", ErrUnsupportedFormat)
	}

	dec, err := buildDecrypter(data)
	if err != nil {
		return "", err
	}

	doc := scanPDFObjects(data, dec)
	if len(doc.objs) == 0 {
		return "", fmt.Errorf("%w：解析不出任何 PDF 对象", ErrUnsupportedFormat)
	}
	doc.expandObjectStreams()

	pages := doc.collectPages()
	if len(pages) == 0 {
		return "", ErrPDFNoTextLayer
	}

	var sb strings.Builder
	for _, page := range pages {
		sb.WriteString(doc.extractPageText(page, 0))
		sb.WriteString(doc.extractAnnotationText(page, 0))
		sb.WriteString("\n")
	}
	text := normalizePDFText(sb.String())
	if strings.TrimSpace(text) == "" {
		return "", ErrPDFNoTextLayer
	}
	return text, nil
}

// scanPDFObjects 顺序扫描 `N G obj ... endobj`，不依赖 xref。
// 有加密时（空用户密码）在这里把流与字符串就地解密。
func scanPDFObjects(data []byte, dec *pdfDecrypter) *pdfDoc {
	doc := &pdfDoc{objs: map[int]any{}}
	i := 0
	for i < len(data) {
		idx := bytes.Index(data[i:], []byte("obj"))
		if idx < 0 {
			break
		}
		p := i + idx
		j := p - 1
		for j >= 0 && isPDFWhite(data[j]) {
			j--
		}
		genEnd := j + 1
		for j >= 0 && data[j] >= '0' && data[j] <= '9' {
			j--
		}
		genStart := j + 1
		if genStart == genEnd {
			i = p + 3
			continue
		}
		if _, err := strconv.Atoi(string(data[genStart:genEnd])); err != nil {
			i = p + 3
			continue
		}
		gen, err := strconv.Atoi(string(data[genStart:genEnd]))
		if err != nil {
			i = p + 3
			continue
		}
		for j >= 0 && isPDFWhite(data[j]) {
			j--
		}
		numEnd := j + 1
		for j >= 0 && data[j] >= '0' && data[j] <= '9' {
			j--
		}
		numStart := j + 1
		if numStart == numEnd {
			i = p + 3
			continue
		}
		if numStart > 0 {
			prev := data[numStart-1]
			if !isPDFWhite(prev) && !isPDFDelim(prev) {
				i = p + 3
				continue
			}
		}
		num, err := strconv.Atoi(string(data[numStart:numEnd]))
		if err != nil {
			i = p + 3
			continue
		}

		sc := &pdfScanner{buf: data, pos: p + 3}
		obj, err := sc.parseObject()
		if err != nil {
			i = p + 3
			continue
		}
		sc.skipSpace()
		if stream, ok := readStream(sc, obj); ok {
			stream.data = dec.decrypt(num, gen, stream.data)
			obj = stream
		} else {
			obj = decryptObjectStrings(obj, dec, num, gen)
		}
		// 同一个对象号出现多次 = 增量更新（扫描仪/Office 二次保存很常见），
		// PDF 规范里生效的是最后一份定义，所以这里必须「后写覆盖」。
		if _, exists := doc.objs[num]; !exists {
			doc.order = append(doc.order, num)
		}
		doc.objs[num] = obj
		if sc.pos > i {
			i = sc.pos
		} else {
			i = p + 3
		}
	}
	return doc
}

// readStream 在对象后面读到 `stream ... endstream` 时把裸字节挂上。
func readStream(sc *pdfScanner, obj any) (*pdfStream, bool) {
	dict, ok := obj.(pdfDict)
	if !ok {
		return nil, false
	}
	if !sc.hasKeyword("stream") {
		return nil, false
	}
	if sc.pos < len(sc.buf) && sc.buf[sc.pos] == '\r' {
		sc.pos++
	}
	if sc.pos < len(sc.buf) && sc.buf[sc.pos] == '\n' {
		sc.pos++
	}
	start := sc.pos
	end := -1
	if length := intOf(dict["Length"]); length > 0 && start+length <= len(sc.buf) {
		if idx := bytes.Index(sc.buf[start+length:], []byte("endstream")); idx >= 0 && idx < 8 {
			end = start + length
		}
	}
	if end < 0 {
		if idx := bytes.Index(sc.buf[start:], []byte("endstream")); idx >= 0 {
			end = start + idx
			for end > start && (sc.buf[end-1] == '\n' || sc.buf[end-1] == '\r') {
				end--
			}
		}
	}
	if end < 0 {
		return nil, false
	}
	raw := sc.buf[start:end]
	sc.pos = end
	sc.hasKeyword("endstream")
	return &pdfStream{dict: dict, data: raw}, true
}

// hasKeyword 判断当前位置（跳过空白）是否以某个关键字开头，是则消费它。
func (s *pdfScanner) hasKeyword(kw string) bool {
	save := s.pos
	s.skipSpace()
	if s.pos+len(kw) <= len(s.buf) && string(s.buf[s.pos:s.pos+len(kw)]) == kw {
		next := s.pos + len(kw)
		if next >= len(s.buf) || isPDFWhite(s.buf[next]) || isPDFDelim(s.buf[next]) {
			s.pos = next
			return true
		}
	}
	s.pos = save
	return false
}

// expandObjectStreams 把 /Type /ObjStm 里压缩的对象展开到对象表（现代 PDF 大量使用）。
func (d *pdfDoc) expandObjectStreams() {
	for _, num := range append([]int(nil), d.order...) {
		st, ok := d.objs[num].(*pdfStream)
		if !ok || nameOf(st.dict["Type"]) != "ObjStm" {
			continue
		}
		data, err := d.decodeStream(st)
		if err != nil {
			continue
		}
		n := intOf(st.dict["N"])
		first := intOf(st.dict["First"])
		if n <= 0 || first < 0 || first > len(data) {
			continue
		}
		sc := &pdfScanner{buf: data}
		type pair struct{ num, off int }
		pairs := make([]pair, 0, n)
		for len(pairs) < n {
			sc.skipSpace()
			if sc.pos >= len(data) {
				break
			}
			numObj, err := sc.parseObject()
			if err != nil {
				break
			}
			offObj, err := sc.parseObject()
			if err != nil {
				break
			}
			pairs = append(pairs, pair{num: intOf(numObj), off: intOf(offObj)})
		}
		for _, pr := range pairs {
			if _, exists := d.objs[pr.num]; exists {
				continue
			}
			pos := first + pr.off
			if pos <= 0 || pos >= len(data) {
				continue
			}
			inner := &pdfScanner{buf: data, pos: pos}
			obj, err := inner.parseObject()
			if err != nil {
				continue
			}
			d.objs[pr.num] = obj
			d.order = append(d.order, pr.num)
		}
	}
}

// ---------- 取值助手 ----------

func (d *pdfDoc) resolve(obj any) any {
	for i := 0; i < 8; i++ {
		ref, ok := obj.(pdfRef)
		if !ok {
			return obj
		}
		next, ok := d.objs[int(ref)]
		if !ok {
			return nil
		}
		obj = next
	}
	return obj
}

func (d *pdfDoc) dictOf(obj any) pdfDict {
	switch t := d.resolve(obj).(type) {
	case pdfDict:
		return t
	case *pdfStream:
		return t.dict
	}
	return nil
}

func (d *pdfDoc) arrayOf(obj any) pdfArray {
	if arr, ok := d.resolve(obj).(pdfArray); ok {
		return arr
	}
	return nil
}

func nameOf(obj any) string {
	if n, ok := obj.(pdfName); ok {
		return string(n)
	}
	return ""
}

func intOf(obj any) int { return int(floatOf(obj)) }

func floatOf(obj any) float64 {
	switch t := obj.(type) {
	case float64:
		return t
	case pdfKeyword:
		if f, err := strconv.ParseFloat(string(t), 64); err == nil {
			return f
		}
	}
	return 0
}

func stringOf(obj any) string {
	if s, ok := obj.(pdfString); ok {
		return string(s)
	}
	return ""
}

// boolOf 读布尔值：词法器把 true/false 归到关键字里，不能直接做类型断言。
func boolOf(obj any) bool {
	switch t := obj.(type) {
	case bool:
		return t
	case pdfKeyword:
		return string(t) == "true"
	}
	return false
}

// ---------- 滤镜 ----------

var errImageOrUnknownFilter = errors.New("非文本滤镜")

func (d *pdfDoc) filterNames(obj any) []string {
	switch t := d.resolve(obj).(type) {
	case pdfName:
		return []string{string(t)}
	case pdfArray:
		out := make([]string, 0, len(t))
		for _, it := range t {
			if n := nameOf(d.resolve(it)); n != "" {
				out = append(out, n)
			}
		}
		return out
	}
	return nil
}

func (d *pdfDoc) decodeStream(st *pdfStream) ([]byte, error) {
	data := st.data
	filters := d.filterNames(st.dict["Filter"])
	for i, f := range filters {
		var parm pdfDict
		switch t := d.resolve(st.dict["DecodeParms"]).(type) {
		case pdfDict:
			parm = t
		case pdfArray:
			if i < len(t) {
				parm = d.dictOf(t[i])
			}
		}
		out, err := applyFilter(f, data)
		if err != nil {
			return nil, err
		}
		data = applyPredictor(out, parm)
	}
	return data, nil
}

func applyFilter(name string, data []byte) ([]byte, error) {
	switch name {
	case "", "FlateDecode", "Fl":
		return inflate(data)
	case "ASCIIHexDecode", "AHx":
		digits := make([]byte, 0, len(data))
		for _, c := range data {
			if c == '>' {
				break
			}
			if isPDFWhite(c) {
				continue
			}
			digits = append(digits, c)
		}
		if len(digits)%2 == 1 {
			digits = append(digits, '0')
		}
		out := make([]byte, 0, len(digits)/2)
		for i := 0; i+1 < len(digits); i += 2 {
			b, err := strconv.ParseUint(string(digits[i:i+2]), 16, 8)
			if err != nil {
				return nil, err
			}
			out = append(out, byte(b))
		}
		return out, nil
	case "ASCII85Decode", "A85":
		src := bytes.TrimSpace(data)
		src = bytes.TrimPrefix(src, []byte("<~"))
		src = bytes.TrimSuffix(src, []byte("~>"))
		out := make([]byte, len(src))
		n, _, err := ascii85.Decode(out, src, true)
		if err != nil && n == 0 {
			return nil, err
		}
		return out[:n], nil
	case "RunLengthDecode", "RL":
		var out []byte
		for i := 0; i < len(data); {
			l := int(data[i])
			i++
			switch {
			case l == 128:
				return out, nil
			case l < 128:
				if i+l+1 > len(data) {
					return out, nil
				}
				out = append(out, data[i:i+l+1]...)
				i += l + 1
			default:
				if i >= len(data) {
					return out, nil
				}
				for j := 0; j < 257-l; j++ {
					out = append(out, data[i])
				}
				i++
			}
		}
		return out, nil
	default:
		// DCTDecode / JPXDecode / CCITTFaxDecode / JBIG2Decode 是图片，没有文本
		return nil, fmt.Errorf("%w：%s", errImageOrUnknownFilter, name)
	}
}

func inflate(data []byte) ([]byte, error) {
	if r, err := zlib.NewReader(bytes.NewReader(data)); err == nil {
		defer r.Close()
		if out, err := io.ReadAll(io.LimitReader(r, 1<<30)); err == nil {
			return out, nil
		}
	}
	// 少数流是裸 deflate（没有 zlib 头）
	r := flate.NewReader(bytes.NewReader(data))
	defer r.Close()
	if out, err := io.ReadAll(io.LimitReader(r, 1<<30)); err == nil {
		return out, nil
	}
	return nil, errors.New("解压失败")
}

// applyPredictor 处理 PNG（>=10）与 TIFF（2）预测器（8 位/分量时生效）。
func applyPredictor(data []byte, parm pdfDict) []byte {
	if parm == nil {
		return data
	}
	pred := intOf(parm["Predictor"])
	if pred <= 1 {
		return data
	}
	colors := intOf(parm["Colors"])
	if colors <= 0 {
		colors = 1
	}
	bpc := intOf(parm["BitsPerComponent"])
	if bpc <= 0 {
		bpc = 8
	}
	columns := intOf(parm["Columns"])
	if columns <= 0 {
		columns = 1
	}
	if bpc != 8 {
		return data
	}
	rowLen := colors * columns
	if rowLen <= 0 {
		return data
	}
	if pred == 2 {
		for row := 0; row+rowLen <= len(data); row += rowLen {
			for i := colors; i < rowLen; i++ {
				data[row+i] += data[row+i-colors]
			}
		}
		return data
	}
	stride := rowLen + 1
	rows := len(data) / stride
	out := make([]byte, 0, rows*rowLen)
	prev := make([]byte, rowLen)
	for r := 0; r < rows; r++ {
		ft := data[r*stride]
		line := make([]byte, rowLen)
		copy(line, data[r*stride+1:(r+1)*stride])
		switch ft {
		case 1:
			for i := colors; i < rowLen; i++ {
				line[i] += line[i-colors]
			}
		case 2:
			for i := 0; i < rowLen; i++ {
				line[i] += prev[i]
			}
		case 3:
			for i := 0; i < rowLen; i++ {
				left := 0
				if i >= colors {
					left = int(line[i-colors])
				}
				line[i] += byte((left + int(prev[i])) / 2)
			}
		case 4:
			for i := 0; i < rowLen; i++ {
				left, upLeft := 0, 0
				if i >= colors {
					left = int(line[i-colors])
					upLeft = int(prev[i-colors])
				}
				line[i] += byte(paeth(left, int(prev[i]), upLeft))
			}
		}
		out = append(out, line...)
		prev = line
	}
	return out
}

func paeth(a, b, c int) int {
	p := a + b - c
	pa, pb, pc := abs(p-a), abs(p-b), abs(p-c)
	if pa <= pb && pa <= pc {
		return a
	}
	if pb <= pc {
		return b
	}
	return c
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// ---------- 页树 ----------

// collectPages 按阅读顺序返回页面（未取到页树时退化为按对象号排序）。
func (d *pdfDoc) collectPages() []pdfDict {
	var catalog pdfDict
	for _, num := range append([]int(nil), d.order...) {
		if nameOf(d.dictOf(d.objs[num])["Type"]) == "Catalog" {
			catalog = d.dictOf(d.objs[num])
			break
		}
	}
	var pages []pdfDict
	seen := map[int]bool{}
	if catalog != nil {
		d.walkPages(catalog["Pages"], nil, &pages, seen, 0)
	}
	if len(pages) == 0 {
		nums := append([]int(nil), d.order...)
		sort.Ints(nums)
		for _, num := range nums {
			if dd := d.dictOf(d.objs[num]); nameOf(dd["Type"]) == "Page" {
				pages = append(pages, dd)
			}
		}
	}
	return pages
}

func (d *pdfDoc) walkPages(node any, inherited pdfDict, out *[]pdfDict, seen map[int]bool, depth int) {
	if depth > 64 {
		return
	}
	if ref, ok := node.(pdfRef); ok {
		if seen[int(ref)] {
			return
		}
		seen[int(ref)] = true
	}
	dict := d.dictOf(node)
	if dict == nil {
		return
	}
	res := inherited
	if r := d.dictOf(dict["Resources"]); r != nil {
		res = r
	}
	switch nameOf(dict["Type"]) {
	case "Page":
		*out = append(*out, dictWithResources(dict, res))
	case "Pages":
		for _, kid := range d.arrayOf(dict["Kids"]) {
			d.walkPages(kid, res, out, seen, depth+1)
		}
	default:
		if arr := d.arrayOf(dict["Kids"]); len(arr) > 0 {
			for _, kid := range arr {
				d.walkPages(kid, res, out, seen, depth+1)
			}
			return
		}
		if dict["Contents"] != nil {
			*out = append(*out, dictWithResources(dict, res))
		}
	}
}

// dictWithResources 把页面没写全的 /Resources 用页树继承下来的补上。
func dictWithResources(page, res pdfDict) pdfDict {
	if page["Resources"] != nil || res == nil {
		return page
	}
	merged := pdfDict{}
	for k, v := range page {
		merged[k] = v
	}
	merged["Resources"] = res
	return merged
}

// ---------- 文本抽取 ----------

type pdfFont struct {
	twoByte bool
	cmap    map[uint32]string
	simple  map[byte]string
	// legacy：编码名直接写明字符集（GBK-EUC-H / ETen-B5-H / UniGB-UCS2-H …）时的解码器。
	// 中文排版软件（方正、扫描全能王等）大量用 GBK-EUC-H 且不带 ToUnicode，
	// 这类字体必须按编码名解，否则整页抽出来是空的。
	legacy encoding.Encoding
	// gidToUnicode：内嵌 TrueType 的 cmap 反查表（ToUnicode 不全时兜底）。
	// 只有 CIDToGIDMap 为 Identity 时才能直接用码当 GID。
	gidToUnicode map[uint16]rune
	cidToGID     map[uint16]uint16
}

// legacyEncodingFor 把 PDF 的编码名映射到 x/text 解码器（认不出返回 nil）。
func legacyEncodingFor(name string) encoding.Encoding {
	switch {
	case name == "":
		return nil
	case strings.Contains(name, "GBK"), strings.Contains(name, "GB-EUC"), strings.Contains(name, "GBpc-EUC"):
		return simplifiedchinese.GBK
	case strings.Contains(name, "GB2312"), strings.Contains(name, "EUC-CN"):
		// GB2312 是 GBK 的子集，用 GBK 解不会错
		return simplifiedchinese.GBK
	case strings.Contains(name, "ETen"), strings.Contains(name, "B5"), strings.Contains(name, "Big5"):
		return traditionalchinese.Big5
	case strings.Contains(name, "KSCms-UHC"), strings.Contains(name, "KSC-EUC"),
		strings.Contains(name, "KSCpc-EUC"), strings.Contains(name, "EUC-KR"):
		return korean.EUCKR
	case strings.Contains(name, "RKSJ"), strings.Contains(name, "90ms"), strings.Contains(name, "90pv"),
		strings.Contains(name, "EUC-H"), strings.Contains(name, "EUC-V"):
		return japanese.ShiftJIS
	case strings.Contains(name, "UCS2"), strings.Contains(name, "UTF16"), strings.Contains(name, "UniGB"),
		strings.Contains(name, "UniJIS"), strings.Contains(name, "UniKS"), strings.Contains(name, "UniCNS"):
		return unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM)
	}
	return nil
}

func (d *pdfDoc) loadFonts(res pdfDict) map[string]*pdfFont {
	out := map[string]*pdfFont{}
	if res == nil {
		return out
	}
	fontRes := d.dictOf(res["Font"])
	if fontRes == nil {
		return out
	}
	for name, ref := range fontRes {
		if objNum, ok := ref.(pdfRef); ok {
			if cached, hit := d.fontCache[int(objNum)]; hit {
				out[name] = cached
				continue
			}
		}
		fdict := d.dictOf(ref)
		if fdict == nil {
			continue
		}
		font := &pdfFont{simple: baseSimpleTable()}
		if nameOf(fdict["Subtype"]) == "Type0" {
			font.twoByte = true
		}
		if tu := d.streamOf(fdict["ToUnicode"]); tu != nil {
			if data, err := d.decodeStream(tu); err == nil {
				font.cmap = parseToUnicodeCMap(data)
			}
		}
		d.attachEmbeddedFont(fdict, font)
		if encName := nameOf(fdict["Encoding"]); encName != "" {
			font.legacy = legacyEncodingFor(encName)
		}
		if enc := d.dictOf(fdict["Encoding"]); enc != nil {
			if nameOf(enc["BaseEncoding"]) == "MacRomanEncoding" {
				font.simple = macRomanTable()
			}
			applyDifferences(font.simple, enc["Differences"])
		}
		out[name] = font
		if objNum, ok := ref.(pdfRef); ok {
			if d.fontCache == nil {
				d.fontCache = map[int]*pdfFont{}
			}
			d.fontCache[int(objNum)] = font
		}
	}
	return out
}

// attachEmbeddedFont 把内嵌 TrueType 的 cmap 反查表挂到字体上。
// Type0 要再看一层 DescendantFonts 才能拿到 FontDescriptor / CIDToGIDMap。
func (d *pdfDoc) attachEmbeddedFont(fdict pdfDict, font *pdfFont) {
	fontDict := fdict
	if nameOf(fdict["Subtype"]) == "Type0" {
		descendants := d.arrayOf(fdict["DescendantFonts"])
		if len(descendants) == 0 {
			return
		}
		child := d.dictOf(descendants[0])
		if child == nil {
			return
		}
		fontDict = child
		// /CIDToGIDMap 是流时按 2 字节/项读；/Identity 或缺省等价于「码=GID」
		if st := d.streamOf(child["CIDToGIDMap"]); st != nil {
			if raw, err := d.decodeStream(st); err == nil {
				m := make(map[uint16]uint16, len(raw)/2)
				for i := 0; i+1 < len(raw); i += 2 {
					m[uint16(i/2)] = uint16(raw[i])<<8 | uint16(raw[i+1])
				}
				font.cidToGID = m
			}
		}
	}
	descriptor := d.dictOf(fontDict["FontDescriptor"])
	if descriptor == nil {
		return
	}
	file := d.streamOf(descriptor["FontFile2"])
	if file == nil {
		return
	}
	raw, err := d.decodeStream(file)
	if err != nil {
		return
	}
	_, gidToUnicode := parseSFNTCMaps(raw)
	font.gidToUnicode = gidToUnicode
}

func (d *pdfDoc) streamOf(obj any) *pdfStream {
	if st, ok := d.resolve(obj).(*pdfStream); ok {
		return st
	}
	return nil
}

func (f *pdfFont) lookup(code uint32) (string, bool) {
	if f == nil {
		return "", false
	}
	if f.cmap != nil {
		if s, ok := f.cmap[code]; ok {
			return s, true
		}
	}
	if !f.twoByte && code < 256 {
		if s, ok := f.simple[byte(code)]; ok {
			return s, true
		}
	}
	return "", false
}

func (f *pdfFont) decode(raw []byte) string {
	var sb strings.Builder
	// 有编码名兜底且没有 ToUnicode：整串按该字符集解（GBK 等是变长编码，不能逐码点查）
	if f != nil && f.cmap == nil && f.legacy != nil {
		if out, err := f.legacy.NewDecoder().Bytes(raw); err == nil {
			return string(out)
		}
	}
	// 没有任何线索时也不能直接当 WinAnsi：国产排版软件常把 GBK 双字节塞进声明为
	// WinAnsiEncoding 的字体里（实测《雍正皇帝》整本书都这样、黑体/仿宋_GB2312）。
	if f == nil || (f.cmap == nil && !f.twoByte) {
		if guess, ok := decodeLegacyGuess(raw); ok {
			return guess
		}
	}
	if f != nil && f.twoByte {
		for i := 0; i+1 < len(raw); i += 2 {
			code := uint32(raw[i])<<8 | uint32(raw[i+1])
			if s, ok := f.lookup(code); ok {
				sb.WriteString(s)
				continue
			}
			// ToUnicode 缺口：用内嵌字体的 cmap 反查（CIDToGIDMap 为 Identity 时 GID 就是码）
			if f.gidToUnicode != nil {
				gid := uint16(code)
				if f.cidToGID != nil {
					if mapped, ok := f.cidToGID[uint16(code)]; ok {
						gid = mapped
					} else {
						continue
					}
				}
				if r, ok := f.gidToUnicode[gid]; ok && r > 0 {
					sb.WriteRune(r)
				}
			}
		}
		return sb.String()
	}
	for _, b := range raw {
		if s, ok := f.lookup(uint32(b)); ok {
			sb.WriteString(s)
		} else if b >= 32 && b < 127 {
			sb.WriteByte(b)
		}
	}
	return sb.String()
}

// decodeLegacyGuess 在没有 ToUnicode、也没有可用编码名时，按字节特征猜一次 CJK 编码。
//
// 判据（宁可不解也不要把西文糟蹋了）：
//   - 高字节占比 ≥ 1/3（GBK 中文几乎全是高字节，西文重音词远达不到）；
//   - 候选解码无替换字符，且结果里"像文字"的字符（CJK/字母/数字/标点）占比 ≥ 0.75。
func decodeLegacyGuess(raw []byte) (string, bool) {
	if len(raw) == 0 {
		return "", false
	}
	high := 0
	for _, b := range raw {
		if b >= 0x80 {
			high++
		}
	}
	if high*3 < len(raw) {
		return "", false
	}
	best, bestScore := "", 0.0
	for _, enc := range []encoding.Encoding{
		simplifiedchinese.GBK,
		traditionalchinese.Big5,
		japanese.ShiftJIS,
	} {
		out, err := enc.NewDecoder().Bytes(raw)
		if err != nil {
			continue
		}
		s := string(out)
		if strings.ContainsRune(s, '\uFFFD') {
			continue
		}
		score := textLikeness(s)
		if score > bestScore {
			best, bestScore = s, score
		}
	}
	if bestScore < 0.75 {
		return "", false
	}
	return best, true
}

// textLikeness 估算一个字符串"像正常文字"的比例。
func textLikeness(s string) float64 {
	runes := []rune(s)
	if len(runes) == 0 {
		return 0
	}
	good := 0
	for _, r := range runes {
		switch {
		case r >= 0x4E00 && r <= 0x9FFF: // CJK 基本区
			good += 2
		case r >= 0x3400 && r <= 0x4DBF: // 扩展 A
			good += 2
		case r >= 0x3000 && r <= 0x303F: // CJK 标点
			good++
		case r >= 0xFF00 && r <= 0xFFEF: // 全角
			good++
		case r >= 0x30 && r <= 0x39, r >= 0x41 && r <= 0x5A, r >= 0x61 && r <= 0x7A, r == ' ':
			good++
		}
	}
	// CJK 计 2 分，所以上限是 2*len
	return float64(good) / float64(2*len(runes))
}

// pageContent 拼接页面的内容流（图片流跳过）。
func (d *pdfDoc) pageContent(page pdfDict) []byte {
	var out []byte
	appendStream := func(obj any) {
		st := d.streamOf(obj)
		if st == nil {
			return
		}
		if sub := nameOf(st.dict["Subtype"]); sub == "Image" || sub == "ImageMask" {
			return
		}
		data, err := d.decodeStream(st)
		if err != nil {
			return
		}
		out = append(out, data...)
		out = append(out, '\n')
	}
	switch t := d.resolve(page["Contents"]).(type) {
	case *pdfStream:
		appendStream(page["Contents"])
	case pdfArray:
		for _, it := range t {
			appendStream(it)
		}
	}
	return out
}

// extractPageText 按内容流算子抽文本；Form XObject 会递归展开（里面也常有正文）。
func (d *pdfDoc) extractPageText(page pdfDict, depth int) string {
	content := d.pageContent(page)
	if len(content) == 0 {
		return ""
	}
	res := d.dictOf(page["Resources"])
	fonts := d.loadFonts(res)
	xobjects := d.dictOf(res["XObject"])

	var (
		sb       strings.Builder
		stack    []any
		font     *pdfFont
		fontSize = 12.0
		lastY    = math.NaN()
		lineY    = 0.0
		sc       = &pdfScanner{buf: content}
	)
	// 换行判定：y 变化超过「半个字号」才算换行。
	// 注意不能用固定阈值 —— OCR 文本层（每个字一个 BT/Tm/Tj/ET）里相邻字的 y 会差 1-2pt，
	// 用 1pt 的阈值会把每个字都切成一行（实测踩过）。
	newlineIfMoved := func(y float64) {
		threshold := math.Max(1.0, 0.5*math.Abs(fontSize))
		if !math.IsNaN(lastY) && math.Abs(y-lastY) > threshold {
			sb.WriteString("\n")
		}
		lastY = y
	}
	show := func(raw []byte) {
		if len(raw) == 0 {
			return
		}
		sb.WriteString(font.decode(raw))
	}

	for sc.pos < len(content) {
		obj, err := sc.parseObject()
		if err != nil {
			break
		}
		kw, ok := obj.(pdfKeyword)
		if !ok {
			if len(stack) < 64 {
				stack = append(stack, obj)
			}
			continue
		}
		switch string(kw) {
		case "BT":
			// 不重置 lastY：文本块之间也要比 y（OCR 文本层是逐字一个 BT/ET）
		case "ET":
			// 不在 ET 处换行：OCR 文本层会逐字 BT/ET，
			// 换行的职责交给 Tm/Td/T* 按 y 位移判断。
		case "Tf":
			if len(stack) >= 2 {
				font = fonts[nameOf(stack[len(stack)-2])]
				fontSize = floatOf(stack[len(stack)-1])
			}
		case "Td", "TD":
			if len(stack) >= 2 {
				lineY += floatOf(stack[len(stack)-1])
				newlineIfMoved(lineY)
			}
		case "Tm":
			if len(stack) >= 6 {
				lineY = floatOf(stack[len(stack)-1])
				newlineIfMoved(lineY)
			}
		case "T*":
			lineY -= fontSize
			sb.WriteString("\n")
			lastY = lineY
		case "Tj":
			if len(stack) >= 1 {
				show([]byte(stringOf(stack[len(stack)-1])))
			}
		case "TJ":
			if len(stack) >= 1 {
				if arr, ok := stack[len(stack)-1].(pdfArray); ok {
					for _, it := range arr {
						if s, ok := it.(pdfString); ok {
							show([]byte(s))
						}
					}
				}
			}
		case "'":
			sb.WriteString("\n")
			if len(stack) >= 1 {
				show([]byte(stringOf(stack[len(stack)-1])))
			}
		case "\"":
			sb.WriteString("\n")
			if len(stack) >= 1 {
				show([]byte(stringOf(stack[len(stack)-1])))
			}
		case "Do":
			if depth < maxFormDepth && len(stack) >= 1 && xobjects != nil {
				xo := d.streamOf(xobjects[nameOf(stack[len(stack)-1])])
				if xo != nil && nameOf(xo.dict["Subtype"]) == "Form" {
					inner := pdfDict{}
					for k, v := range xo.dict {
						inner[k] = v
					}
					if innerRes := d.dictOf(xo.dict["Resources"]); innerRes != nil {
						inner["Resources"] = innerRes
					} else if page["Resources"] != nil {
						inner["Resources"] = page["Resources"]
					}
					inner["Contents"] = xo
					inner["Type"] = pdfName("Page")
					sb.WriteString(d.extractPageText(inner, depth+1))
				}
			}
		}
		stack = stack[:0]
	}
	return sb.String()
}

// extractAnnotationText 抽注释外观流里的文本。
//
// 为什么需要它：批量扫描出来的 PDF 常把「机密 / 严禁复制」这类水印做成注释（/Annots → /AP → /N），
// 正文内容流里只有一张图 —— 不读注释外观流就会误判成"纯扫描件"。
func (d *pdfDoc) extractAnnotationText(page pdfDict, depth int) string {
	if depth > maxFormDepth {
		return ""
	}
	annots := d.arrayOf(page["Annots"])
	if len(annots) == 0 {
		return ""
	}
	var sb strings.Builder
	appendAppearance := func(obj any, fallbackRes pdfDict) {
		st := d.streamOf(obj)
		if st == nil {
			return
		}
		inner := pdfDict{"Contents": st, "Type": pdfName("Page")}
		switch {
		case st.dict["Resources"] != nil:
			inner["Resources"] = st.dict["Resources"]
		case fallbackRes != nil:
			inner["Resources"] = fallbackRes
		case page["Resources"] != nil:
			inner["Resources"] = page["Resources"]
		}
		sb.WriteString(d.extractPageText(inner, depth+1))
	}
	for _, a := range annots {
		ad := d.dictOf(a)
		if ad == nil {
			continue
		}
		res := d.dictOf(ad["Resources"])
		ap := d.dictOf(ad["AP"])
		if ap == nil {
			continue
		}
		if st := d.streamOf(ap["N"]); st != nil {
			appendAppearance(ap["N"], res)
			continue
		}
		// /N 是「状态 → 外观流」的字典时，取第一个可用状态
		if states := d.dictOf(ap["N"]); states != nil {
			keys := make([]string, 0, len(states))
			for k := range states {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				appendAppearance(states[k], res)
			}
		}
	}
	return sb.String()
}

// ---------- ToUnicode CMap ----------

func parseToUnicodeCMap(data []byte) map[uint32]string {
	out := map[uint32]string{}
	sc := &pdfScanner{buf: data}
	for sc.pos < len(data) {
		obj, err := sc.parseObject()
		if err != nil {
			break
		}
		kw, ok := obj.(pdfKeyword)
		if !ok {
			continue
		}
		switch string(kw) {
		case "beginbfchar":
			for {
				src, err := sc.parseObject()
				if err != nil {
					break
				}
				if k, ok := src.(pdfKeyword); ok && string(k) == "endbfchar" {
					break
				}
				dst, err := sc.parseObject()
				if err != nil {
					break
				}
				if k, ok := dst.(pdfKeyword); ok && string(k) == "endbfchar" {
					break
				}
				if s := utf16BEToString([]byte(stringOf(dst))); s != "" {
					out[hexToUint(src)] = s
				}
			}
		case "beginbfrange":
			for {
				loObj, err := sc.parseObject()
				if err != nil {
					break
				}
				if k, ok := loObj.(pdfKeyword); ok && string(k) == "endbfrange" {
					break
				}
				hiObj, err := sc.parseObject()
				if err != nil {
					break
				}
				dst, err := sc.parseObject()
				if err != nil {
					break
				}
				lo, hi := hexToUint(loObj), hexToUint(hiObj)
				if hi < lo || hi-lo > 0xFFFF {
					continue
				}
				switch t := dst.(type) {
				case pdfString:
					base := []byte(t)
					for c := lo; c <= hi; c++ {
						out[c] = utf16BEToString(incrementUTF16BE(base, c-lo))
					}
				case pdfArray:
					for i, it := range t {
						s, ok := it.(pdfString)
						if !ok {
							continue
						}
						if code := lo + uint32(i); code <= hi {
							out[code] = utf16BEToString([]byte(s))
						}
					}
				}
			}
		}
	}
	return out
}

func hexToUint(obj any) uint32 {
	s, ok := obj.(pdfString)
	if !ok {
		return 0
	}
	var v uint32
	for _, b := range s {
		v = v<<8 | uint32(b)
	}
	return v
}

// incrementUTF16BE 把 UTF-16BE 基址加 n（按码元从低位进位，符合 CMap 规范）。
func incrementUTF16BE(base []byte, n uint32) []byte {
	if len(base) == 0 {
		return base
	}
	out := append([]byte(nil), base...)
	units := len(out) / 2
	add := n
	for i := units - 1; i >= 0 && add > 0; i-- {
		cur := uint32(out[i*2])<<8 | uint32(out[i*2+1])
		sum := cur + add
		out[i*2] = byte(sum >> 8)
		out[i*2+1] = byte(sum)
		add = sum >> 16
	}
	return out
}

func utf16BEToString(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	if len(raw)%2 == 1 {
		raw = raw[:len(raw)-1]
	}
	units := make([]uint16, 0, len(raw)/2)
	for i := 0; i+1 < len(raw); i += 2 {
		units = append(units, uint16(raw[i])<<8|uint16(raw[i+1]))
	}
	return string(utf16.Decode(units))
}

// ---------- 简单字体编码表 ----------

func baseSimpleTable() map[byte]string {
	table := make(map[byte]string, 160)
	for b := 0x20; b < 0x7F; b++ {
		table[byte(b)] = string(rune(b))
	}
	for b := 0xA0; b < 0x100; b++ {
		table[byte(b)] = string(rune(b))
	}
	for b, r := range winAnsiSpecials {
		table[b] = string(r)
	}
	return table
}

var winAnsiSpecials = map[byte]rune{
	0x80: '\u20ac', 0x82: '\u201a', 0x83: '\u0192', 0x84: '\u201e', 0x85: '\u2026',
	0x86: '\u2020', 0x87: '\u2021', 0x88: '\u02c6', 0x89: '\u2030', 0x8a: '\u0160',
	0x8b: '\u2039', 0x8c: '\u0152', 0x8e: '\u017d', 0x91: '\u2018', 0x92: '\u2019',
	0x93: '\u201c', 0x94: '\u201d', 0x95: '\u2022', 0x96: '\u2013', 0x97: '\u2014',
	0x98: '\u02dc', 0x99: '\u2122', 0x9a: '\u0161', 0x9b: '\u203a', 0x9c: '\u0153',
	0x9e: '\u017e', 0x9f: '\u0178',
}

func macRomanTable() map[byte]string {
	table := make(map[byte]string, 128)
	for b := 0x20; b < 0x7F; b++ {
		table[byte(b)] = string(rune(b))
	}
	high := []rune{
		'\u00c4', '\u00c5', '\u00c7', '\u00c9', '\u00d1', '\u00d6', '\u00dc', '\u00e1',
		'\u00e0', '\u00e2', '\u00e4', '\u00e3', '\u00e5', '\u00e7', '\u00e9', '\u00e8',
		'\u00ea', '\u00eb', '\u00ed', '\u00ec', '\u00ee', '\u00ef', '\u00f1', '\u00f3',
		'\u00f2', '\u00f4', '\u00f6', '\u00f5', '\u00fa', '\u00f9', '\u00fb', '\u00fc',
	}
	for i, r := range high {
		table[byte(0x80+i)] = string(r)
	}
	return table
}

func applyDifferences(table map[byte]string, obj any) {
	arr, ok := obj.(pdfArray)
	if !ok {
		return
	}
	code := -1
	for _, it := range arr {
		switch t := it.(type) {
		case float64:
			code = int(t)
		case pdfName:
			if code >= 0 && code < 256 {
				if r, ok := glyphNameToRune(string(t)); ok {
					table[byte(code)] = string(r)
				}
			}
			code++
		}
	}
}

// glyphNameToRune 只认常见字形名；认不出就保持原样，避免把表写坏。
func glyphNameToRune(name string) (rune, bool) {
	if name == "" {
		return 0, false
	}
	if len(name) == 1 {
		return rune(name[0]), true
	}
	if strings.HasPrefix(name, "uni") && len(name) >= 7 {
		if v, err := strconv.ParseUint(name[3:7], 16, 32); err == nil {
			return rune(v), true
		}
	}
	switch name {
	case "space":
		return ' ', true
	case "quotesingle":
		return '\'', true
	case "quotedbl":
		return '"', true
	case "endash":
		return '\u2013', true
	case "emdash":
		return '\u2014', true
	case "bullet":
		return '\u2022', true
	case "ellipsis":
		return '\u2026', true
	case "quoteright":
		return '\u2019', true
	case "quoteleft":
		return '\u2018', true
	}
	return 0, false
}

// ---------- 后处理 ----------

func normalizePDFText(raw string) string {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, "\r", "\n")
	raw = strings.ReplaceAll(raw, "\x00", "")
	lines := strings.Split(raw, "\n")
	out := make([]string, 0, len(lines))
	blank := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			blank++
			if blank > 1 {
				continue
			}
			out = append(out, "")
			continue
		}
		blank = 0
		out = append(out, line)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}
