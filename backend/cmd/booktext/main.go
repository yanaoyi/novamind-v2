// booktext 把一本书（txt/docx/pdf/epub/mobi）的正文抽到 stdout。
//
// 用途：与同书其他格式的抽取结果做对照校验（见 scripts/check-book-extract.sh）；
// 产品链路不调用它，导入接口走的是同一份 parser.ParseByFilename。
package main

import (
	"fmt"
	"os"
	"sort"

	"github.com/yanaoyi/novamindv2/backend/internal/parser"
)

func main() {
	args := os.Args[1:]
	stats := false
	list := 0
	if len(args) > 0 && (args[0] == "-stats" || args[0] == "--stats") {
		stats = true
		args = args[1:]
	}
	if len(args) > 1 && (args[0] == "-chapters" || args[0] == "--chapters") {
		list = 20
		args = args[1:]
	}
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "用法: booktext [-stats] <file>   （支持 .txt/.md/.docx/.pdf/.epub/.mobi）")
		os.Exit(2)
	}
	name := args[0]
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
	// -stats：只报切分统计（章节数、空章、最长/平均），用于批量体检切分质量而不用导库
	if stats {
		chapters := parser.SplitChapters(text)
		if list > 0 {
			for i, c := range chapters {
				if i >= list {
					break
				}
				fmt.Printf("  #%d %q %d 字\n", i+1, c.Title, len([]rune(c.Content)))
			}
		}
		sizes := make([]int, 0, len(chapters))
		empties := 0
		for _, c := range chapters {
			n := len([]rune(c.Content))
			sizes = append(sizes, n)
			if n < 12 {
				empties++
			}
		}
		sort.Ints(sizes)
		sum := 0
		for _, n := range sizes {
			sum += n
		}
		avg, max := 0, 0
		if len(sizes) > 0 {
			avg, max = sum/len(sizes), sizes[len(sizes)-1]
		}
		fmt.Printf("encoding=%s runes=%d chapters=%d empty=%d min=%d avg=%d max=%d\n",
			encoding, len([]rune(text)), len(chapters), empties,
			firstLen(sizes), avg, max)
		return
	}
	fmt.Fprintf(os.Stderr, "OK encoding=%s runes=%d\n", encoding, len([]rune(text)))
	fmt.Print(text)
}

func firstLen(sorted []int) int {
	if len(sorted) == 0 {
		return 0
	}
	return sorted[0]
}
