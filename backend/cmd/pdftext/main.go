// pdftext 把 PDF 的文本层抽到 stdout。
//
// 用途：与 PyMuPDF 的抽取结果做对照校验（见 scripts/check-pdf-extract.sh）。
// 产品链路不调用它，导入接口走的是同一份 parser.ParsePDF。
package main

import (
	"fmt"
	"os"

	"github.com/yanaoyi/novamindv2/backend/internal/parser"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "用法: pdftext <file.pdf>   （PDF_DEBUG=1 时额外打印解析诊断）")
		os.Exit(2)
	}
	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "读取文件失败: %v\n", err)
		os.Exit(2)
	}
	if os.Getenv("PDF_DEBUG") != "" {
		parser.DebugPDF(data, os.Stderr)
	}
	text, err := parser.ParsePDF(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERR: %v\n", err)
		os.Exit(1)
	}
	fmt.Print(text)
}
