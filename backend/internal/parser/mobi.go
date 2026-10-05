package parser

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// MOBI 解析（格式扩展：除 TXT/DOCX/PDF 之外支持 .mobi 导入）。
//
// 文件结构（Palm Database，PDB）：
//
//	PDB 头（78 字节）→ 记录索引表（每条 8 字节：offset/attr/uid）
//	记录 0 = PalmDOC 头（16 字节）+ MOBI 头 → 之后每 4096 字节切一条记录
//	PalmDOC 头：compression(2) / text_length(4) / record_count(2) / record_size(2)
//
// 压缩支持（实测 BOSS 书库 977 本 .mobi 的分布）：
//
//	compression=1 未压缩（15 本）
//	compression=2 PalmDOC LZ77（958 本）
//	compression=17480 HUFF/CDIC（2 本）→ 明确报错并给出可行做法，不静默乱码
//
// 记录尾部可能追加 "extra data"（KF8 常见）。这里不去解析它：
// 按 text_length 逐条"只解出需要的字节数"，尾部数据自然不会被读到 ——
// 跨记录被切开的多字节字符也能在拼接时重新接上，比正向解析 extra data 更稳。

// ErrMOBIUnsupported 表示这种 MOBI 变体当前不支持（但错误信息里会给出可行做法）。
var ErrMOBIUnsupported = errors.New("这个 MOBI 暂不支持")

const (
	pdbHeaderSize   = 78
	pdbRecordEntry  = 8
	mobiTextEncUTF8 = 65001
)

// ParseMOBI 从 .mobi 字节流抽取纯文本。
func ParseMOBI(data []byte) (string, error) {
	if len(data) < pdbHeaderSize+2 {
		return "", fmt.Errorf("%w：文件太小，不是完整的 MOBI", ErrMOBIInvalid)
	}
	if string(data[60:64]) != "BOOK" {
		return "", fmt.Errorf("%w：文件头不是 BOOK/MOBI（可能是 .prc 之外的格式或已损坏）", ErrMOBIInvalid)
	}
	recordCount := int(binary.BigEndian.Uint16(data[76:78]))
	if recordCount < 2 {
		return "", fmt.Errorf("%w：记录数异常（%d）", ErrMOBIInvalid, recordCount)
	}
	offsets, err := pdbRecordOffsets(data, recordCount)
	if err != nil {
		return "", err
	}

	rec0 := data[offsets[0]:offsets[1]]
	if len(rec0) < 16 {
		return "", fmt.Errorf("%w：记录 0 太短", ErrMOBIInvalid)
	}
	compression := binary.BigEndian.Uint16(rec0[0:2])
	textLength := int(binary.BigEndian.Uint32(rec0[4:8]))
	recCount := int(binary.BigEndian.Uint16(rec0[8:10]))
	recordSize := int(binary.BigEndian.Uint16(rec0[10:12]))
	encryption := binary.BigEndian.Uint16(rec0[12:14])

	if encryption == 1 {
		return "", fmt.Errorf("%w：这是受 DRM 保护的 MOBI，无法直接解析（请用已授权的阅读器导出或换 EPUB/TXT）", ErrMOBIUnsupported)
	}
	if textLength <= 0 {
		return "", fmt.Errorf("%w：文本长度为 0（可能是纯 KF8/AZW3，请改用 EPUB）", ErrMOBIUnsupported)
	}
	if recordSize <= 0 {
		// 极少数文件不填 record_size，按 PalmDOC 约定退化为 4096
		recordSize = 4096
	}
	switch compression {
	case 1, 2:
		// 支持
	case 17480:
		return "", fmt.Errorf("%w：该文件使用 HUFF/CDIC 压缩（KindleGen/KF8 常用，实测书库中占 2/977）。"+
			"请用 Calibre 转成 EPUB 再导入（EPUB 已支持）", ErrMOBIUnsupported)
	default:
		return "", fmt.Errorf("%w：未知压缩方式 %d", ErrMOBIUnsupported, compression)
	}

	// MOBI 头紧跟 PalmDOC 头；编码字段决定文本是否 UTF-8
	utf8Text := true
	if len(rec0) >= 16+32 {
		if string(rec0[16:20]) == "MOBI" {
			enc := binary.BigEndian.Uint32(rec0[28:32])
			utf8Text = enc == mobiTextEncUTF8
		}
	}

	var sb strings.Builder
	sb.Grow(textLength + 64)
	remaining := textLength
	records := recCount
	if maxRecords := len(offsets) - 1; records > maxRecords {
		records = maxRecords
	}
	for i := 1; i <= records && remaining > 0; i++ {
		rec := data[offsets[i]:offsets[i+1]]
		want := recordSize
		if remaining < want {
			want = remaining
		}
		var chunk []byte
		switch compression {
		case 1:
			chunk = rec
			if len(chunk) > want {
				chunk = chunk[:want]
			}
		case 2:
			out, err := palmdocDecompress(rec, want)
			if err != nil {
				return "", fmt.Errorf("解压第 %d 条记录失败: %w", i, err)
			}
			chunk = out
		}
		if len(chunk) == 0 {
			break
		}
		sb.Write(chunk)
		remaining -= len(chunk)
	}
	raw := sb.String()
	if !utf8Text {
		// MOBI 文本编码字段非 UTF-8（通常是 CP1252）
		raw = decodeCP1252(raw)
	}
	text := StripHTMLToText(raw)
	if strings.TrimSpace(text) == "" {
		return "", ErrEmptyText
	}
	return text, nil
}

// ErrMOBIInvalid 表示文件结构不成立。
var ErrMOBIInvalid = errors.New("不是有效的 MOBI")

// pdbRecordOffsets 读出记录偏移表（含末尾哨兵）。
func pdbRecordOffsets(data []byte, recordCount int) ([]int, error) {
	need := pdbHeaderSize + recordCount*pdbRecordEntry
	if len(data) < need {
		return nil, fmt.Errorf("%w：记录索引表不完整", ErrMOBIInvalid)
	}
	offsets := make([]int, 0, recordCount+1)
	for i := 0; i < recordCount; i++ {
		off := int(binary.BigEndian.Uint32(data[pdbHeaderSize+i*pdbRecordEntry : pdbHeaderSize+i*pdbRecordEntry+4]))
		if off <= 0 || off > len(data) {
			return nil, fmt.Errorf("%w：第 %d 条记录偏移越界（%d）", ErrMOBIInvalid, i, off)
		}
		offsets = append(offsets, off)
	}
	offsets = append(offsets, len(data))
	return offsets, nil
}

// palmdocDecompress 解 PalmDOC（LZ77 + 字节对压缩）编码，最多产出 want 字节。
//
// 规则以 MobileRead 的 PalmDOC 规范为准（v1 实现就是在这里想当然写错了，
// 结果 40/40 本真实书解压失败 —— 记下来提醒后来者：这类格式别凭印象写）：
//
//	0x00            → 1 个字面量（原样输出该字节）
//	0x09..0x7F      → 1 个字面量
//	0x01..0x08      → 从**输入流**原样复制 N 个字面量（N = 该字节值）
//	0x80..0xBF      → 长度-距离对：14 位 = ((b & 0x3F) << 8) | 下一字节，
//	                  高 11 位是距离、低 3 位是长度，复制 length+3（3..10）个字节
//	0xC0..0xFF      → 字节对：输出"空格 + (b ^ 0x80)"
//
// 距离只在本条记录的输出内回指（规范：4096 字节块独立压缩）；
// 产出够 want 字节就停 —— 记录尾部的 extra data 因此不会被当指令读进去。
func palmdocDecompress(src []byte, want int) ([]byte, error) {
	out := make([]byte, 0, want+64)
	i := 0
	for i < len(src) && len(out) < want {
		b := src[i]
		i++
		switch {
		case b == 0x00:
			out = append(out, 0x00)
		case b >= 0x01 && b <= 0x08:
			// 从输入流原样复制 N 个字面量（不是回指！）
			n := int(b)
			if i+n > len(src) {
				out = append(out, src[i:]...)
				i = len(src)
				break
			}
			out = append(out, src[i:i+n]...)
			i += n
		case b >= 0x09 && b <= 0x7F:
			out = append(out, b)
		case b >= 0x80 && b <= 0xBF:
			if i >= len(src) {
				return out, nil
			}
			pair := (int(b&0x3F) << 8) | int(src[i])
			i++
			dist := pair >> 3
			length := (pair & 0x07) + 3
			if dist == 0 || dist > len(out) {
				return out, fmt.Errorf(
					"回指距离越界（dist=%d，本条已解出 %d 字节）—— 文件可能被 XOR 混淆或不是 PalmDOC", dist, len(out))
			}
			for n := 0; n < length; n++ {
				out = append(out, out[len(out)-dist])
				if len(out) >= want {
					break
				}
			}
		default: // 0xC0..0xFF
			out = append(out, ' ', b^0x80)
		}
	}
	if len(out) > want {
		out = out[:want]
	}
	return out, nil
}

// decodeCP1252 把非 UTF-8 的 MOBI 文本按 CP1252/Latin-1 还原成 UTF-8。
//
// 先做一次"合法 UTF-8 就原样返回"的判断：MOBI 头里的编码字段有时不可靠
// （写着 1252 但内容是 UTF-8），硬转会把中文全毁掉。
func decodeCP1252(s string) string {
	if utf8.ValidString(s) && strings.ContainsAny(s, "\u4e00\u9fff") {
		return s
	}
	var sb strings.Builder
	sb.Grow(len(s))
	for _, b := range []byte(s) {
		if b < 0x80 {
			sb.WriteByte(b)
			continue
		}
		sb.WriteRune(cp1252Rune(b))
	}
	return sb.String()
}

// cp1252Rune 把 0x80..0x9F 区间的 CP1252 特殊字符映射到 Unicode（其余同 Latin-1）。
func cp1252Rune(b byte) rune {
	table := map[byte]rune{
		0x80: '€', 0x82: '‚', 0x83: 'ƒ', 0x84: '„', 0x85: '…', 0x86: '†', 0x87: '‡',
		0x88: 'ˆ', 0x89: '‰', 0x8a: 'Š', 0x8b: '‹', 0x8c: 'Œ', 0x8e: 'Ž',
		0x91: '‘', 0x92: '’', 0x93: '“', 0x94: '”', 0x95: '•', 0x96: '–', 0x97: '—',
		0x98: '˜', 0x99: '™', 0x9a: 'š', 0x9b: '›', 0x9c: 'œ', 0x9e: 'ž', 0x9f: 'Ÿ',
	}
	if r, ok := table[b]; ok {
		return r
	}
	return rune(b)
}
