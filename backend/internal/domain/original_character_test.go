package domain

import (
	"errors"
	"strings"
	"testing"
)

func TestCharacterDNAValidate(t *testing.T) {
	cases := []struct {
		name    string
		dna     CharacterDNA
		wantErr bool
	}{
		{
			name: "全空 DNA 合法（维度可选）",
			dna:  CharacterDNA{},
		},
		{
			name: "边界值 0 与 100 合法",
			dna: CharacterDNA{
				Personality: DNADimension{Text: "冷", Weight: 0},
				Values:      DNADimension{Text: "守信", Weight: 100},
			},
		},
		{
			name:    "超过 100 非法",
			dna:     CharacterDNA{Personality: DNADimension{Weight: 101}},
			wantErr: true,
		},
		{
			name:    "负数非法",
			dna:     CharacterDNA{SpeechStyle: DNADimension{Weight: -1}},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.dna.Validate()
			if tc.wantErr && !errors.Is(err, ErrDNAWeightOutOfRange) {
				t.Fatalf("期望 ErrDNAWeightOutOfRange，实际 %v", err)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("期望通过，实际 %v", err)
			}
		})
	}
}

func TestCharacterDNADimensionsCoversAllFields(t *testing.T) {
	// 11 个维度（SPEC.md §6.2）一个都不能漏
	if got := len(CharacterDNA{}.Dimensions()); got != 11 {
		t.Fatalf("DNA 维度应为 11 个，实际 %d 个", got)
	}
}

func TestOriginalCharacterNormalizeAndValidate(t *testing.T) {
	c := &OriginalCharacter{
		Name:    "  林默  ",
		Aliases: []string{" 小默 ", "", "小默", "林先生"},
	}
	c.Normalize()

	if c.Name != "林默" {
		t.Errorf("姓名未清洗: %q", c.Name)
	}
	if len(c.Aliases) != 2 || c.Aliases[0] != "小默" || c.Aliases[1] != "林先生" {
		t.Errorf("别名应去空白去重，实际 %v", c.Aliases)
	}
	if c.Importance != 3 {
		t.Errorf("重要度应默认 3，实际 %d", c.Importance)
	}
	if c.Source != SourceManual {
		t.Errorf("来源应默认 MANUAL，实际 %s", c.Source)
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("校验应通过: %v", err)
	}

	// 姓名必填
	empty := &OriginalCharacter{Name: "   "}
	if err := empty.Validate(); !errors.Is(err, ErrCharacterNameRequired) {
		t.Errorf("空姓名应报错，实际 %v", err)
	}

	// 姓名超长
	long := &OriginalCharacter{Name: strings.Repeat("名", 121)}
	if err := long.Validate(); !errors.Is(err, ErrCharacterNameTooLong) {
		t.Errorf("超长姓名应报错，实际 %v", err)
	}

	// 重要度越界
	bad := &OriginalCharacter{Name: "x", Importance: 9}
	if err := bad.Validate(); !errors.Is(err, ErrCharacterImportance) {
		t.Errorf("重要度越界应报错，实际 %v", err)
	}
}

func TestCharacterRelationshipValidateAndNormalize(t *testing.T) {
	valid := &CharacterRelationship{
		SourceCharacterID: "a",
		TargetCharacterID: "b",
		RelationType:      RelationFriend,
	}
	valid.Normalize()
	if valid.Strength != 50 {
		t.Errorf("强度应默认 50，实际 %d", valid.Strength)
	}
	if valid.Source != SourceManual {
		t.Errorf("来源应默认 MANUAL，实际 %s", valid.Source)
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("校验应通过: %v", err)
	}

	self := &CharacterRelationship{SourceCharacterID: "a", TargetCharacterID: "a", RelationType: RelationFriend}
	if err := self.Validate(); !errors.Is(err, ErrRelationSelfReference) {
		t.Errorf("自环应报错，实际 %v", err)
	}

	badType := &CharacterRelationship{SourceCharacterID: "a", TargetCharacterID: "b", RelationType: "cousin"}
	if err := badType.Validate(); !errors.Is(err, ErrRelationTypeInvalid) {
		t.Errorf("非法关系类型应报错，实际 %v", err)
	}

	badStrength := &CharacterRelationship{
		SourceCharacterID: "a", TargetCharacterID: "b",
		RelationType: RelationFriend, Strength: 200,
	}
	if err := badStrength.Validate(); !errors.Is(err, ErrRelationStrength) {
		t.Errorf("强度越界应报错，实际 %v", err)
	}
}

func TestRelationTypeValid(t *testing.T) {
	for _, ty := range []RelationType{
		RelationFamily, RelationFriend, RelationLover, RelationEnemy, RelationMentor,
		RelationStudent, RelationColleague, RelationRival, RelationOrganization, RelationOther,
	} {
		if !ty.Valid() {
			t.Errorf("%q 应合法", ty)
		}
	}
	if RelationType("").Valid() || RelationType("COUSIN").Valid() {
		t.Error("空/未知关系类型不应合法")
	}
}
