package parser

// PDF 解析诊断（给 scripts/check-pdf-extract.sh 与排障用）。
// 产品链路不调用；它的存在是为了让"这本 PDF 为什么没抽到字"能一眼看出来。

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
)

// DebugPDF 打印一份 PDF 的解析过程概览。
func DebugPDF(data []byte, w io.Writer) {
	dec, decErr := buildDecrypter(data)
	switch {
	case decErr != nil:
		fmt.Fprintf(w, "加密: 有，但无法解（%v）\n", decErr)
	case dec != nil:
		fmt.Fprintf(w, "加密: 有，已按空密码解开（key %d 字节，AES=%v）\n", len(dec.key), dec.aes)
	default:
		fmt.Fprintln(w, "加密: 无")
	}
	doc := scanPDFObjects(data, dec)
	fmt.Fprintf(w, "对象总数（顶层）: %d\n", len(doc.objs))
	doc.expandObjectStreams()
	fmt.Fprintf(w, "展开对象流后: %d\n", len(doc.objs))
	objStm, streams := 0, 0
	for _, v := range doc.objs {
		if st, ok := v.(*pdfStream); ok {
			streams++
			if nameOf(st.dict["Type"]) == "ObjStm" {
				objStm++
			}
		}
	}
	fmt.Fprintf(w, "流对象: %d（其中对象流 %d）\n", streams, objStm)

	pages := doc.collectPages()
	fmt.Fprintf(w, "页面数: %d\n", len(pages))
	onlyPage := 0
	if raw := os.Getenv("PDF_DEBUG_PAGE"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil {
			onlyPage = v
		}
	}
	for i, page := range pages {
		if onlyPage > 0 && i+1 != onlyPage {
			continue
		}
		if onlyPage == 0 && os.Getenv("PDF_DEBUG_ALL") == "" && i >= 5 {
			fmt.Fprintf(w, "  …（其余 %d 页略）\n", len(pages)-i)
			break
		}
		content := doc.pageContent(page)
		res := doc.dictOf(page["Resources"])
		fonts := doc.loadFonts(res)
		names := make([]string, 0, len(fonts))
		for name, f := range fonts {
			desc := []string{}
			if f.twoByte {
				desc = append(desc, "Type0/双字节")
			}
			if f.cmap != nil {
				desc = append(desc, fmt.Sprintf("ToUnicode=%d 条", len(f.cmap)))
			} else {
				desc = append(desc, "无ToUnicode")
			}
			if f.legacy != nil {
				desc = append(desc, "有编码兜底")
			}
			if f.gidToUnicode != nil {
				desc = append(desc, fmt.Sprintf("内嵌字体反查=%d 条", len(f.gidToUnicode)))
			}
			if f.cidToGID != nil {
				desc = append(desc, fmt.Sprintf("CIDToGIDMap=%d 项", len(f.cidToGID)))
			}
			names = append(names, fmt.Sprintf("%s(%s)", name, strings.Join(desc, ",")))
		}
		sort.Strings(names)
		xobjects := d_xobjectNames(doc, res)
		text := doc.extractPageText(page, 0)
		fmt.Fprintf(w, "  第 %d 页: 内容流 %d 字节, 字体 %d 个 [%s], XObject [%s], 抽出 %d 字\n",
			i+1, len(content), len(fonts), strings.Join(names, " "), strings.Join(xobjects, " "), len([]rune(text)))
		if onlyPage > 0 {
			// 指定页时把字体字典细节也打出来，方便判断是不是 ToUnicode 没解析全
			for name := range fonts {
				fdict := doc.dictOf(doc.dictOf(res["Font"])[name])
				keys := make([]string, 0, len(fdict))
				for k := range fdict {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				fmt.Fprintf(w, "     字体 %s: %s\n", name, strings.Join(keys, " "))
				if tu := doc.streamOf(fdict["ToUnicode"]); tu != nil {
					raw, _ := doc.decodeStream(tu)
					fmt.Fprintf(w, "       ToUnicode 原始 %d 字节: %s\n", len(raw), strings.ReplaceAll(string(raw[:min(len(raw), 300)]), "\n", " "))
				}
			}
		}
	}
}

func d_xobjectNames(doc *pdfDoc, res pdfDict) []string {
	if res == nil {
		return nil
	}
	xo := doc.dictOf(res["XObject"])
	out := make([]string, 0, len(xo))
	for name, ref := range xo {
		sub := ""
		if st := doc.streamOf(ref); st != nil {
			sub = nameOf(st.dict["Subtype"])
		} else if dd := doc.dictOf(ref); dd != nil {
			sub = nameOf(dd["Subtype"])
		}
		out = append(out, name+":"+sub)
	}
	sort.Strings(out)
	return out
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
