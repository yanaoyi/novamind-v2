package domain

import (
	"errors"
	"testing"
)

func TestWorldRuleNormalizeAndValidate(t *testing.T) {
	rule := &WorldRule{Name: "  灵力不可凭空产生  ", Category: " 力量体系 "}
	rule.Normalize()

	if rule.Name != "灵力不可凭空产生" || rule.Category != "力量体系" {
		t.Fatalf("未清洗输入: %+v", rule)
	}
	if rule.Importance != 3 {
		t.Errorf("重要度应默认 3，实际 %d", rule.Importance)
	}
	if err := rule.Validate(); err != nil {
		t.Fatalf("校验应通过: %v", err)
	}

	if err := (&WorldRule{Name: "  "}).Validate(); !errors.Is(err, ErrWorldRuleNameEmpty) {
		t.Errorf("空名称应报错，实际 %v", err)
	}
	if err := (&WorldRule{Name: "x", Importance: 6}).Validate(); !errors.Is(err, ErrWorldRuleImportance) {
		t.Errorf("重要度越界应报错，实际 %v", err)
	}
}

func TestLocationNormalizeAndValidate(t *testing.T) {
	empty := "   "
	loc := &Location{Name: "  青云城 ", Type: " 城市 ", ParentLocationID: &empty}
	loc.Normalize()

	if loc.Name != "青云城" || loc.Type != "城市" {
		t.Fatalf("未清洗输入: %+v", loc)
	}
	if loc.ParentLocationID != nil {
		t.Error("空白上级 ID 应被归一为 nil")
	}
	if err := loc.Validate(); err != nil {
		t.Fatalf("校验应通过: %v", err)
	}

	if err := (&Location{Name: " "}).Validate(); !errors.Is(err, ErrLocationNameEmpty) {
		t.Errorf("空名称应报错，实际 %v", err)
	}

	self := "loc-1"
	bad := &Location{ID: "loc-1", Name: "自己", ParentLocationID: &self}
	if err := bad.Validate(); !errors.Is(err, ErrLocationSelfParent) {
		t.Errorf("自环应报错，实际 %v", err)
	}
}

func TestFactionNormalizeAndValidate(t *testing.T) {
	f := &Faction{Name: " 天枢阁 ", Type: " 宗门 ", Goals: "  垄断灵矿 "}
	f.Normalize()
	if f.Name != "天枢阁" || f.Type != "宗门" || f.Goals != "垄断灵矿" {
		t.Fatalf("未清洗输入: %+v", f)
	}
	if err := f.Validate(); err != nil {
		t.Fatalf("校验应通过: %v", err)
	}
	if err := (&Faction{Name: "   "}).Validate(); !errors.Is(err, ErrFactionNameEmpty) {
		t.Errorf("空名称应报错，实际 %v", err)
	}
}

func TestWorldNormalize(t *testing.T) {
	w := &OriginalWorld{Name: " 九州 ", Description: " 架空大陆 "}
	w.Normalize()
	if w.Name != "九州" || w.Description != "架空大陆" {
		t.Fatalf("未清洗输入: %+v", w)
	}
}
