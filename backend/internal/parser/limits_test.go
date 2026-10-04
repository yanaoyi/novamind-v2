package parser

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestCheckCompressionRatioRejectsBombs(t *testing.T) {
	// 正常 docx：几十 KB 压缩、几百 KB 解压
	if err := checkCompressionRatio(20_000, 200_000, "word/document.xml"); err != nil {
		t.Errorf("正常文件不应被拒: %v", err)
	}
	// 声明解压后超上限
	if err := checkCompressionRatio(1_000, maxDecompressedBytes+1, "word/document.xml"); !errors.Is(err, ErrDecompressedTooLarge) {
		t.Errorf("超上限应被拒，实际 %v", err)
	}
	// 压缩比过高（全零数据的典型特征）
	if err := checkCompressionRatio(1_000, 10_000_000, "word/document.xml"); !errors.Is(err, ErrDecompressedTooLarge) {
		t.Errorf("压缩比过高应被拒，实际 %v", err)
	}
}

func TestLimitReaderFailsInsteadOfTruncating(t *testing.T) {
	// 恰好读满上限且真的结束：应正常返回
	exact := bytes.Repeat([]byte("a"), 100)
	out, err := io.ReadAll(limitReader(bytes.NewReader(exact), 100, "测试流"))
	if err != nil {
		t.Fatalf("恰好读满不应报错: %v", err)
	}
	if len(out) != 100 {
		t.Fatalf("应读到 100 字节，实际 %d", len(out))
	}

	// 超过上限：必须报错，不能静默返回半截内容
	tooBig := bytes.Repeat([]byte("a"), 101)
	if _, err := io.ReadAll(limitReader(bytes.NewReader(tooBig), 100, "测试流")); !errors.Is(err, ErrDecompressedTooLarge) {
		t.Fatalf("超过上限应报错，实际 %v", err)
	}
}

func TestChunkByLengthOffsetsMatchContent(t *testing.T) {
	text := strings.Repeat("甲", 10) + strings.Repeat("乙", 10) + strings.Repeat("丙", 10)
	chapters := chunkByLength(text, 10)
	if len(chapters) != 3 {
		t.Fatalf("应切成 3 段，实际 %d", len(chapters))
	}
	// 每段的字节区间应该正好对应它在原文里的位置（用于章节原文跳转/定位）
	for i, ch := range chapters {
		start := int(ch.Start)
		end := int(ch.End)
		if end > len(text) || start > end {
			t.Fatalf("第 %d 段偏移非法: %d-%d（原文 %d 字节）", i+1, start, end, len(text))
		}
		if got := strings.TrimSpace(text[start:end]); got != ch.Content {
			t.Errorf("第 %d 段偏移与内容不一致: 偏移取到 %q，内容字段是 %q", i+1, got, ch.Content)
		}
	}
}
