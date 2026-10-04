package service

import (
	"testing"

	"github.com/yanaoyi/novamindv2/backend/internal/domain"
)

func TestClampImportance(t *testing.T) {
	cases := []struct {
		in   int
		want int
	}{
		{0, 3},   // 模型漏字段 → 中等重要度
		{-2, 3},  // 负数 → 中等
		{1, 1},   // 合法下界
		{3, 3},   // 合法
		{5, 5},   // 合法上界
		{6, 5},   // 越界上界 → 收到 5
		{100, 5}, // 明显越界
	}
	for _, c := range cases {
		if got := clampImportance(c.in); got != c.want {
			t.Errorf("clampImportance(%d) = %d, 期望 %d", c.in, got, c.want)
		}
	}
}

func TestClampDNAKeepsDimensionsAndBoundsWeights(t *testing.T) {
	in := domain.CharacterDNA{
		Personality: domain.DNADimension{Text: "克制", Weight: 120},
		Ability:     domain.DNADimension{Text: "剑术", Weight: -5},
		SpeechStyle: domain.DNADimension{Text: "短句", Weight: 60},
	}

	out := clampDNA(in)

	if out.Personality.Weight != 100 {
		t.Errorf("越界权重应收敛到 100，实际 %d", out.Personality.Weight)
	}
	if out.Ability.Weight != 0 {
		t.Errorf("负权重应收敛到 0，实际 %d", out.Ability.Weight)
	}
	if out.SpeechStyle.Weight != 60 {
		t.Errorf("合法权重不应被改动，实际 %d", out.SpeechStyle.Weight)
	}
	if out.Personality.Text != "克制" || out.SpeechStyle.Text != "短句" {
		t.Errorf("文本不应被改动: %+v", out)
	}
	// 11 个维度一个都不能丢
	if len(out.Dimensions()) != 11 {
		t.Errorf("维度数应为 11，实际 %d", len(out.Dimensions()))
	}
}
