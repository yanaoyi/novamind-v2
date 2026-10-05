package parser

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
)

// EPUB 解析（格式扩展：除 TXT/DOCX/PDF 之外支持 .epub 导入）。
//
// EPUB 本质是一个 zip：
//   META-INF/container.xml → 指向 .opf（OPF 描述清单）
//   OPF 里 <manifest> 给 id→文件，<spine> 给阅读顺序
//   → 按 spine 顺序读 XHTML，剥标签取正文（htmltext.go）
//
// 只依赖标准库，与 DOCX/PDF 解析器一致；zip 炸弹防护复用 limits.go 的检查。

// ErrEPUBInvalid 表示这个文件不是可用的 EPUB。
var ErrEPUBInvalid = errors.New("不是有效的 EPUB")

const (
	// maxEPUBEntries 限制参与解析的正文文件数（防止"一万个空文件"式的构造）。
	maxEPUBEntries = 4000
	// maxEPUBTextBytes 限制所有正文解压后的总量。
	maxEPUBTextBytes = 256 << 20
)

// container.xml 的最小结构（只要 rootfile 的 full-path）。
type epubContainer struct {
	Rootfiles []struct {
		FullPath  string `xml:"full-path,attr"`
		MediaType string `xml:"media-type,attr"`
	} `xml:"rootfiles>rootfile"`
}

type epubPackage struct {
	Manifest []struct {
		ID        string `xml:"id,attr"`
		Href      string `xml:"href,attr"`
		MediaType string `xml:"media-type,attr"`
	} `xml:"manifest>item"`
	Spine []struct {
		IDRef string `xml:"idref,attr"`
	} `xml:"spine>itemref"`
}

// ParseEPUB 从 .epub 字节流抽取纯文本（按 spine 顺序拼接各 XHTML）。
func ParseEPUB(data []byte) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("%w：不是合法的 zip 包（%v）", ErrEPUBInvalid, err)
	}
	index := newZipIndex(zr)

	containerPath, container, err := findEPUBContainer(index)
	if err != nil {
		return "", err
	}
	_ = containerPath

	opfPath, err := readOPFPath(index, container)
	if err != nil {
		return "", err
	}
	raw, err := index.read(opfPath, 8<<20)
	if err != nil {
		return "", fmt.Errorf("%w：读取 %s 失败（%v）", ErrEPUBInvalid, opfPath, err)
	}
	var pkg epubPackage
	if err := xml.Unmarshal(raw, &pkg); err != nil {
		return "", fmt.Errorf("%w：解析 %s 失败（%v）", ErrEPUBInvalid, opfPath, err)
	}
	hrefByID := map[string]string{}
	for _, item := range pkg.Manifest {
		if item.ID != "" {
			hrefByID[item.ID] = item.Href
		}
	}

	// spine 顺序 = 阅读顺序；没有 spine 时退化为"按 manifest 里的 xhtml/html 文件排序"
	ordered := make([]string, 0, len(pkg.Spine))
	for _, ref := range pkg.Spine {
		if href, ok := hrefByID[ref.IDRef]; ok {
			ordered = append(ordered, href)
		}
	}
	if len(ordered) == 0 {
		fallback := make([]string, 0, len(pkg.Manifest))
		for _, item := range pkg.Manifest {
			mt := strings.ToLower(item.MediaType)
			ext := strings.ToLower(path.Ext(item.Href))
			if mt == "application/xhtml+xml" || ext == ".xhtml" || ext == ".html" || ext == ".htm" {
				fallback = append(fallback, item.Href)
			}
		}
		sort.Strings(fallback)
		ordered = fallback
	}
	if len(ordered) == 0 {
		return "", fmt.Errorf("%w：找到 OPF 但里面没有可读的正文文件", ErrEPUBInvalid)
	}

	var sb strings.Builder
	total := 0
	used := 0
	for _, href := range ordered {
		if used >= maxEPUBEntries {
			break
		}
		used++
		entryPath := resolveZipPath(path.Dir(opfPath), href)
		body, err := index.read(entryPath, 32<<20)
		if err != nil {
			// 单个文件读不到不该让整本失败（EPUB 里常有指向不存在文件的 spine 项）
			continue
		}
		text := StripHTMLToText(string(body))
		if strings.TrimSpace(text) == "" {
			continue
		}
		total += len(text)
		if total > maxEPUBTextBytes {
			return "", fmt.Errorf("%w（%v）", ErrDecompressedTooLarge, "EPUB 正文")
		}
		sb.WriteString(text)
		sb.WriteString("\n\n")
	}
	out := trimAll(sb.String())
	if strings.TrimSpace(out) == "" {
		return "", ErrEmptyText
	}
	return out, nil
}

// findEPUBContainer 读 META-INF/container.xml（大小写与路径变体都容忍）。
func findEPUBContainer(index *zipIndex) (string, epubContainer, error) {
	var container epubContainer
	for _, name := range index.names() {
		if !strings.EqualFold(path.Base(name), "container.xml") {
			continue
		}
		raw, err := index.read(name, 1<<20)
		if err != nil {
			continue
		}
		if err := xml.Unmarshal(raw, &container); err != nil {
			continue
		}
		if len(container.Rootfiles) > 0 {
			return name, container, nil
		}
	}
	return "", container, fmt.Errorf("%w：缺少 META-INF/container.xml（可能是 .mobi 改名或已损坏）", ErrEPUBInvalid)
}

// readOPFPath 从 container 里取 OPF 路径；container 缺失时退回按扩展名找 .opf。
func readOPFPath(index *zipIndex, container epubContainer) (string, error) {
	for _, rf := range container.Rootfiles {
		p := resolveZipPath("", rf.FullPath)
		if _, ok := index.lookup(p); ok {
			return p, nil
		}
	}
	for _, name := range index.names() {
		if strings.EqualFold(path.Ext(name), ".opf") {
			return name, nil
		}
	}
	return "", fmt.Errorf("%w：找不到 OPF 描述文件", ErrEPUBInvalid)
}

// resolveZipPath 把 href 解析成 zip 内的绝对路径（处理相对路径、URL 转义、锚点）。
func resolveZipPath(baseDir, href string) string {
	href = strings.TrimSpace(href)
	if i := strings.IndexByte(href, '#'); i >= 0 {
		href = href[:i]
	}
	if i := strings.IndexByte(href, '?'); i >= 0 {
		href = href[:i]
	}
	href = unescapePath(href)
	if strings.HasPrefix(href, "/") {
		return strings.TrimPrefix(path.Clean(href), "/")
	}
	if baseDir == "" || baseDir == "." {
		return path.Clean(href)
	}
	return path.Clean(path.Join(baseDir, href))
}

// unescapePath 还原 href 里的 %XX（zip 内文件名是原始字节，不做转义）。
func unescapePath(s string) string {
	if !strings.ContainsRune(s, '%') {
		return s
	}
	var sb strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '%' && i+2 < len(s) {
			hi, ok1 := hexDigit(s[i+1])
			lo, ok2 := hexDigit(s[i+2])
			if ok1 && ok2 {
				sb.WriteByte(byte(hi<<4 | lo))
				i += 2
				continue
			}
		}
		sb.WriteByte(s[i])
	}
	return sb.String()
}

func hexDigit(c byte) (int, bool) {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0'), true
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10, true
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10, true
	default:
		return 0, false
	}
}

// zipIndex 是 zip 内文件的索引（按原路径与"大小写不敏感路径"双向可查）。
//
// 为什么需要它：EPUB 里的 href 大小写与 zip 条目名不一致是常见问题
// （Windows 上制作的电子书尤其多），严格按字节比较会让整本书读不出来。
type zipIndex struct {
	byName  map[string]*zip.File
	byLower map[string]*zip.File
	files   []*zip.File
}

func newZipIndex(zr *zip.Reader) *zipIndex {
	idx := &zipIndex{
		byName:  make(map[string]*zip.File, len(zr.File)),
		byLower: make(map[string]*zip.File, len(zr.File)),
		files:   zr.File,
	}
	for _, f := range zr.File {
		idx.byName[f.Name] = f
		if _, exists := idx.byLower[strings.ToLower(f.Name)]; !exists {
			idx.byLower[strings.ToLower(f.Name)] = f
		}
	}
	return idx
}

func (z *zipIndex) names() []string {
	out := make([]string, 0, len(z.files))
	for _, f := range z.files {
		out = append(out, f.Name)
	}
	return out
}

func (z *zipIndex) lookup(name string) (*zip.File, bool) {
	if f, ok := z.byName[name]; ok {
		return f, true
	}
	f, ok := z.byLower[strings.ToLower(name)]
	return f, ok
}

// read 读取一个 zip 条目（带压缩比检查与解压上限）。
func (z *zipIndex) read(name string, limit int64) ([]byte, error) {
	f, ok := z.lookup(name)
	if !ok {
		return nil, fmt.Errorf("zip 内没有 %s", name)
	}
	if f.FileInfo().IsDir() {
		return nil, fmt.Errorf("%s 是目录", name)
	}
	if err := checkCompressionRatio(f.CompressedSize64, f.UncompressedSize64, name); err != nil {
		return nil, err
	}
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	body, err := io.ReadAll(limitReader(rc, limit, name))
	if err != nil {
		return nil, err
	}
	return body, nil
}
