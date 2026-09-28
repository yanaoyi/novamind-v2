package parser

import (
	"encoding/binary"
	"testing"
)

// buildTestTTF 手搓一个最小的 TTF：只有表目录 + 一张 cmap(format 4)。
// 段布局：0x0041('A') → GID 1（走 idDelta）；0x4E00-0x4E02 → GID 100/101/102（走 idRangeOffset）。
func buildTestTTF() []byte {
	const segCount = 3 // 两个真实段 + 规范要求的 0xFFFF 结束段
	sub := make([]byte, 16+segCount*8+6)
	binary.BigEndian.PutUint16(sub[0:], 4)
	binary.BigEndian.PutUint16(sub[6:], segCount*2)
	endBase := 14
	startBase := 16 + segCount*2
	deltaBase := startBase + segCount*2
	rangeBase := deltaBase + segCount*2
	glyphBase := rangeBase + segCount*2
	put16 := func(at, v int) { binary.BigEndian.PutUint16(sub[at:], uint16(v)) }
	// endCode
	put16(endBase+0, 0x0041)
	put16(endBase+2, 0x4E02)
	put16(endBase+4, 0xFFFF)
	// startCode
	put16(startBase+0, 0x0041)
	put16(startBase+2, 0x4E00)
	put16(startBase+4, 0xFFFF)
	// idDelta：段0 用 0x41 + (-64) = 1；段2 用 +1（规范做法）
	put16(deltaBase+0, 0xFFC0)
	put16(deltaBase+2, 0)
	put16(deltaBase+4, 1)
	// idRangeOffset：段0/段2 为 0；段1 指向字形数组（相对自己槽位的字节数）
	put16(rangeBase+0, 0)
	put16(rangeBase+2, glyphBase-(rangeBase+2))
	put16(rangeBase+4, 0)
	// glyphIdArray：100 / 101 / 102
	put16(glyphBase+0, 100)
	put16(glyphBase+2, 101)
	put16(glyphBase+4, 102)
	binary.BigEndian.PutUint16(sub[2:], uint16(len(sub)))

	// cmap 表：版本 + 1 个子表记录
	cmap := make([]byte, 4+8+len(sub))
	binary.BigEndian.PutUint16(cmap[2:], 1)
	binary.BigEndian.PutUint16(cmap[4:], 3)  // platformID
	binary.BigEndian.PutUint16(cmap[6:], 1)  // encodingID
	binary.BigEndian.PutUint32(cmap[8:], 12) // offset
	copy(cmap[12:], sub)

	// sfnt 头 + 表目录（只放 cmap 一张表）
	out := make([]byte, 12+16+len(cmap))
	binary.BigEndian.PutUint32(out, 0x00010000)
	binary.BigEndian.PutUint16(out[4:], 1) // numTables
	copy(out[12:16], "cmap")
	binary.BigEndian.PutUint32(out[20:24], uint32(12+16)) // 表偏移
	binary.BigEndian.PutUint32(out[24:28], uint32(len(cmap)))
	copy(out[28:], cmap)
	return out
}

func TestParseSFNTCMaps(t *testing.T) {
	cpToGID, gidToUnicode := parseSFNTCMaps(buildTestTTF())
	if len(cpToGID) == 0 {
		t.Fatal("cmap 没解析出任何映射")
	}
	if got := cpToGID[0x41]; got != 1 {
		t.Errorf("'A' 应映射到 GID 1，实际 %d", got)
	}
	if got := cpToGID[0x4E01]; got != 101 {
		t.Errorf("0x4E01 应映射到 GID 101，实际 %d", got)
	}
	if r, ok := gidToUnicode[101]; !ok || r != 0x4E01 {
		t.Errorf("GID 101 应反查回 0x4E01，实际 %v %v", r, ok)
	}
}

func TestParseSFNTCMapsGarbage(t *testing.T) {
	if cp, _ := parseSFNTCMaps([]byte("not a font")); cp != nil {
		t.Error("垃圾数据应返回 nil")
	}
	if cp, _ := parseSFNTCMaps(nil); cp != nil {
		t.Error("空数据应返回 nil")
	}
}
