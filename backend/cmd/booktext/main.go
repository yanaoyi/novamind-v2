// booktext 把一本书（txt/docx/pdf/epub/mobi）的正文抽到 stdout。
//
// 用途：与同书其他格式的抽取结果做对照校验（见 scripts/check-book-extract.sh）；
// 产品链路不调用它，导入接口走的是同一份 parser.ParseByFilename。
package main

import (
	"fmt"
	"os"

	"github.com/yanaoyi/novamindv2/backend/internal/parser"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "用法: booktext <file>   （支持 .txt/.md/.docx/.pdf/.epub/.mobi）")
		os.Exit(2)
	}
	name := os.Args[1]
	data, err := os.ReadFile(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "读取文件失败: %v\n", err)
		os.Exit(2)
	}
	text, encoding, err := parser.ParseByFilename(name, data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERR: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "OK encoding=%s runes=%d\n", encoding, len([]rune(text)))
	fmt.Print(text)
}
