package parser

import (
	"errors"
	"fmt"
	"io"
)

// ErrDecompressedTooLarge 表示解压后的内容超过安全上限（zip 炸弹防护）。
var ErrDecompressedTooLarge = errors.New("解析失败：文件解压后体积异常（疑似压缩炸弹）")

const (
	// maxDecompressedBytes 是单个压缩流解压后的上限。
	// 正常 docx 的 document.xml、PDF 的单条内容流都远小于这个量级；
	// 50MB 的"全零压缩包"能解出 GB 级数据，正是要拦的场景（审查 P1-8）。
	maxDecompressedBytes = 128 << 20 // 128 MiB
	// maxCompressionRatio 限制压缩比：超过即视为恶意构造。
	maxCompressionRatio = 500
)

// limitReader 把 r 包成"最多读 limit 字节"，读满即报错而不是静默截断。
//
// 静默截断是更危险的做法：调用方拿到半截 XML/正文会当成正常内容入库，
// 而错误会让使用者知道"这个文件不对劲"。
func limitReader(r io.Reader, limit int64, what string) io.Reader {
	return &limitedReader{inner: r, remaining: limit, what: what}
}

type limitedReader struct {
	inner     io.Reader
	remaining int64
	what      string
}

func (l *limitedReader) Read(p []byte) (int, error) {
	if l.remaining <= 0 {
		return l.probe()
	}
	if int64(len(p)) > l.remaining {
		p = p[:l.remaining]
	}
	n, err := l.inner.Read(p)
	l.remaining -= int64(n)
	return n, err
}

// probe 在上限用尽后再读 1 字节：读得到说明内容确实超限（报错），读不到就是正常结束。
// 这样"恰好等于上限"的文件不会被误判成压缩炸弹。
func (l *limitedReader) probe() (int, error) {
	var buf [1]byte
	n, err := l.inner.Read(buf[:])
	if n > 0 {
		return 0, fmt.Errorf("%w（%s）", ErrDecompressedTooLarge, l.what)
	}
	if err == nil {
		err = io.EOF
	}
	return 0, err
}

// checkCompressionRatio 用压缩前后的尺寸做一次快速判断（zip 头里就有这两个值）。
func checkCompressionRatio(compressed, uncompressed uint64, what string) error {
	if uncompressed > maxDecompressedBytes {
		return fmt.Errorf("%w（%s 解压后 %d 字节，超过上限 %d）",
			ErrDecompressedTooLarge, what, uncompressed, maxDecompressedBytes)
	}
	if compressed > 0 && uncompressed/compressed > maxCompressionRatio {
		return fmt.Errorf("%w（%s 压缩比 %d:1）", ErrDecompressedTooLarge, what, uncompressed/compressed)
	}
	return nil
}
