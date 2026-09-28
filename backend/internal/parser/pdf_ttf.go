package parser

// 内嵌 TrueType 字体的 cmap 反查。
//
// 为什么需要：中文年报/公文里的 Type0 字体常只给一份**不完整的 ToUnicode**
// （只覆盖某几页用到的字），剩下的码在 ToUnicode 里查不到。
// 这时唯一的线索是内嵌在 PDF 里的字体本体：它的 cmap 记录「Unicode → 字形编号(GID)」，
// 反过来查 GID 就能把码还原成字（CIDToGIDMap 为 Identity 时 GID 就是码）。
//
// 实测（方盛制药 2017 年报）：0x682→股、0x505→的、0xc06→人、0x1187→资、0x387→本、0x70a→后，全部命中。

import (
	"encoding/binary"
)

const (
	maxCMapEntries = 300000 // 防止畸形字体把内存吃光
)

// parseSFNTCMaps 解析 sfnt（TTF/OTF）里的 cmap 表，返回两张表：
// cpToGID（Unicode → 字形编号）与 gidToUnicode（字形编号 → Unicode）。
func parseSFNTCMaps(data []byte) (map[uint16]rune, map[uint16]rune) {
	if len(data) < 12 {
		return nil, nil
	}
	numTables := int(binary.BigEndian.Uint16(data[4:6]))
	if numTables <= 0 || numTables > 512 {
		return nil, nil
	}
	var cmapOffset, cmapLength int
	for i := 0; i < numTables; i++ {
		off := 12 + i*16
		if off+16 > len(data) {
			return nil, nil
		}
		tag := string(data[off : off+4])
		if tag == "cmap" {
			cmapOffset = int(binary.BigEndian.Uint32(data[off+8 : off+12]))
			cmapLength = int(binary.BigEndian.Uint32(data[off+12 : off+16]))
			break
		}
	}
	if cmapOffset <= 0 || cmapOffset >= len(data) {
		return nil, nil
	}
	end := cmapOffset + cmapLength
	if end > len(data) {
		end = len(data)
	}
	cmap := data[cmapOffset:end]
	if len(cmap) < 4 {
		return nil, nil
	}
	numSubtables := int(binary.BigEndian.Uint16(cmap[2:4]))
	if numSubtables <= 0 || numSubtables > 64 {
		return nil, nil
	}

	cpToGID := map[uint16]rune{}
	best := -1 // 优先 Unicode BMP(3,1) 或 (0,x)，其次 (3,0) 符号
	for i := 0; i < numSubtables; i++ {
		rec := 4 + i*8
		if rec+8 > len(cmap) {
			break
		}
		platform := binary.BigEndian.Uint16(cmap[rec : rec+2])
		encoding := binary.BigEndian.Uint16(cmap[rec+2 : rec+4])
		offset := int(binary.BigEndian.Uint32(cmap[rec+4 : rec+8]))
		if offset <= 0 || offset+2 > len(cmap) {
			continue
		}
		priority := -1
		switch {
		case platform == 3 && encoding == 10:
			priority = 3
		case platform == 3 && encoding == 1:
			priority = 4
		case platform == 0:
			priority = 3
		case platform == 3 && encoding == 0:
			priority = 1
		case platform == 1 && encoding == 0:
			priority = 2
		}
		if priority <= best {
			continue
		}
		pairs := parseCMapSubtable(cmap[offset:])
		if len(pairs) == 0 {
			continue
		}
		best = priority
		cpToGID = pairs
	}
	if len(cpToGID) == 0 {
		return nil, nil
	}
	gidToUnicode := make(map[uint16]rune, len(cpToGID))
	for cp, gid := range cpToGID {
		if _, exists := gidToUnicode[uint16(gid)]; !exists {
			gidToUnicode[uint16(gid)] = rune(cp)
		}
	}
	return cpToGID, gidToUnicode
}

// parseCMapSubtable 支持格式 4（BMP 分段）与格式 12（分组），其余忽略。
func parseCMapSubtable(sub []byte) map[uint16]rune {
	if len(sub) < 4 {
		return nil
	}
	format := binary.BigEndian.Uint16(sub[0:2])
	out := map[uint16]rune{}
	switch format {
	case 4:
		if len(sub) < 14 {
			return nil
		}
		segCountX2 := int(binary.BigEndian.Uint16(sub[6:8]))
		segCount := segCountX2 / 2
		if segCount <= 0 || segCount > 4096 {
			return nil
		}
		endBase := 14
		startBase := endBase + segCountX2 + 2
		deltaBase := startBase + segCountX2
		rangeBase := deltaBase + segCountX2
		if rangeBase+segCountX2 > len(sub) {
			return nil
		}
		count := 0
		for i := 0; i < segCount; i++ {
			endCode := binary.BigEndian.Uint16(sub[endBase+i*2:])
			startCode := binary.BigEndian.Uint16(sub[startBase+i*2:])
			idDelta := int16(binary.BigEndian.Uint16(sub[deltaBase+i*2:]))
			idRangeOffset := binary.BigEndian.Uint16(sub[rangeBase+i*2:])
			if startCode > endCode {
				continue
			}
			for c := int(startCode); c <= int(endCode); c++ {
				if c == 0xFFFF {
					break
				}
				var gid uint16
				if idRangeOffset == 0 {
					gid = uint16(int(c) + int(idDelta))
				} else {
					pos := rangeBase + i*2 + int(idRangeOffset) + (c-int(startCode))*2
					if pos+2 > len(sub) {
						continue
					}
					gid = binary.BigEndian.Uint16(sub[pos:])
					if gid != 0 {
						gid = uint16(int(gid) + int(idDelta))
					}
				}
				if gid != 0 {
					out[uint16(c)] = rune(gid)
				}
				count++
				if count > maxCMapEntries {
					return out
				}
			}
		}
	case 12:
		if len(sub) < 16 {
			return nil
		}
		nGroups := int(binary.BigEndian.Uint32(sub[12:16]))
		if nGroups <= 0 || nGroups > 65536 {
			return nil
		}
		count := 0
		for i := 0; i < nGroups; i++ {
			off := 16 + i*12
			if off+12 > len(sub) {
				return out
			}
			start := binary.BigEndian.Uint32(sub[off : off+4])
			endChar := binary.BigEndian.Uint32(sub[off+4 : off+8])
			startGID := binary.BigEndian.Uint32(sub[off+8 : off+12])
			if start > endChar || endChar-start > maxCMapEntries {
				continue
			}
			for c := start; c <= endChar; c++ {
				gid := startGID + (c - start)
				if c <= 0xFFFF && gid != 0 && gid <= 0xFFFF {
					out[uint16(c)] = rune(gid)
				}
				count++
				if count > maxCMapEntries {
					return out
				}
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
