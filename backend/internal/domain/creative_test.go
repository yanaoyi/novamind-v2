package domain

import (
	"errors"
	"testing"
)

func TestApplyInheritanceScalesWeights(t *testing.T) {
	source := CharacterDNA{
		Personality: DNADimension{Text: "克制、锋利", Weight: 90},
		Values:      DNADimension{Text: "重承诺", Weight: 80},
		Ability:     DNADimension{Text: "会修车", Weight: 50},
		SpeechStyle: DNADimension{Text: "短句", Weight: 40},
	}
	// 规格书 §19 的示例：性格 80%、价值观 80%、语言风格 30%、能力 0%
	rule := InheritanceRule{
		PersonalityWeight: 80, ValueWeight: 80, SpeechWeight: 30, AbilityWeight: 0,
	}

	dna := ApplyInheritance(source, rule)

	if got := dna.Personality.Weight; got != 72 { // 90 × 80%
		t.Errorf("人格权重应为 72，实际 %d", got)
	}
	if dna.Personality.Text != "克制、锋利" {
		t.Errorf("描述应保留，实际 %q", dna.Personality.Text)
	}
	if got := dna.Values.Weight; got != 64 { // 80 × 80%
		t.Errorf("价值观权重应为 64，实际 %d", got)
	}
	if got := dna.SpeechStyle.Weight; got != 12 { // 40 × 30%
		t.Errorf("语言风格权重应为 12，实际 %d", got)
	}
	if dna.Ability.Weight != 0 || dna.Ability.Text != "" {
		t.Errorf("权重 0 的维度应完全不继承，实际 %+v", dna.Ability)
	}
}

func TestInheritanceRuleValidate(t *testing.T) {
	ok := InheritanceRule{PersonalityWeight: 100}
	if err := ok.Validate(); err != nil {
		t.Fatalf("应有合法权重通过: %v", err)
	}

	allZero := InheritanceRule{}
	if err := allZero.Validate(); !errors.Is(err, ErrInheritanceNoDimension) {
		t.Errorf("全 0 权重应报错，实际 %v", err)
	}

	outOfRange := InheritanceRule{PersonalityWeight: 120}
	if err := outOfRange.Validate(); !errors.Is(err, ErrInheritanceWeightInvalid) {
		t.Errorf("越界权重应报错，实际 %v", err)
	}
}

func TestFuseCharactersPicksStrongestSourcePerDimension(t *testing.T) {
	lin := CharacterDNA{
		Personality: DNADimension{Text: "克制", Weight: 95},
		Values:      DNADimension{Text: "重承诺", Weight: 90},
	}
	chen := CharacterDNA{
		Personality: DNADimension{Text: "圆滑", Weight: 60},
		SpeechStyle: DNADimension{Text: "爱开玩笑", Weight: 80},
	}

	// 林默整体 50%、陈述整体 100%
	dna, detail := FuseCharacters([]FusionInput{
		{Character: lin, Fusion: FusionSource{CharacterID: "c1", Name: "林默", Weight: 50}},
		{Character: chen, Fusion: FusionSource{CharacterID: "c2", Name: "陈述", Weight: 100}},
	})

	// 人格：林默 95×50%=47，陈述 60×100%=60 → 取陈述
	if dna.Personality.Weight != 60 || dna.Personality.Text != "圆滑" {
		t.Errorf("人格应取自陈述（60），实际 %+v", dna.Personality)
	}
	// 价值观：只有林默有 → 90×50%=45
	if dna.Values.Weight != 45 {
		t.Errorf("价值观应为 45，实际 %d", dna.Values.Weight)
	}
	// 语言风格：只有陈述有 → 80
	if dna.SpeechStyle.Weight != 80 {
		t.Errorf("语言风格应为 80，实际 %d", dna.SpeechStyle.Weight)
	}

	// 融合说明必须能追溯每个维度来自谁
	byDimension := map[string]FusionAttribution{}
	for _, d := range detail {
		byDimension[d.Dimension] = d
	}
	if byDimension["personality"].FromName != "陈述" {
		t.Errorf("人格应标注来自陈述，实际 %+v", byDimension["personality"])
	}
	if byDimension["values"].FromName != "林默" {
		t.Errorf("价值观应标注来自林默，实际 %+v", byDimension["values"])
	}
	if len(detail) != 3 {
		t.Errorf("应有 3 个维度的来源说明，实际 %d", len(detail))
	}
}

func TestFuseCharactersIgnoresZeroWeightSources(t *testing.T) {
	source := CharacterDNA{Personality: DNADimension{Text: "x", Weight: 100}}
	dna, detail := FuseCharacters([]FusionInput{
		{Character: source, Fusion: FusionSource{CharacterID: "c1", Name: "甲", Weight: 0}},
	})
	if dna.Personality.Weight != 0 || len(detail) != 0 {
		t.Errorf("整体权重为 0 的来源应被忽略，实际 %+v / %v", dna.Personality, detail)
	}
}

func TestCreativeCharacterValidate(t *testing.T) {
	ok := &CreativeCharacter{Name: "林默·二创", SourceType: SourceOriginalInherited}
	ok.Normalize()
	if err := ok.Validate(); err != nil {
		t.Fatalf("应通过校验: %v", err)
	}
	if ok.Importance != 3 {
		t.Errorf("重要度应默认 3，实际 %d", ok.Importance)
	}

	bad := &CreativeCharacter{Name: "x", SourceType: "BOGUS"}
	if err := bad.Validate(); err == nil {
		t.Error("非法来源类型应报错")
	}

	dnaBad := &CreativeCharacter{
		Name: "x", SourceType: SourceNew,
		DNA: CharacterDNA{Personality: DNADimension{Weight: 200}},
	}
	if err := dnaBad.Validate(); !errors.Is(err, ErrDNAWeightOutOfRange) {
		t.Errorf("DNA 越界应报错，实际 %v", err)
	}
}

func TestMappingValidate(t *testing.T) {
	ok := &OriginalCreativeMapping{
		CreativeWorkID: "w", OriginalID: "o", CreativeID: "c", MappingType: MappingInherited,
	}
	if err := ok.Validate(); err != nil {
		t.Fatalf("应通过校验: %v", err)
	}
	bad := &OriginalCreativeMapping{
		CreativeWorkID: "w", OriginalID: "o", CreativeID: "c", MappingType: "MAYBE",
	}
	if err := bad.Validate(); err == nil {
		t.Error("非法映射类型应报错")
	}
}
