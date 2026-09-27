package domain

import (
	"errors"
	"testing"
)

func TestAnalysisStageTaskType(t *testing.T) {
	if got := StageCharacterExtract.TaskType(); got != "analysis_character_extract" {
		t.Errorf("任务类型命名不正确: %s", got)
	}
	if StageWorldExtract.Valid() != true || AnalysisStage("bogus").Valid() {
		t.Error("阶段合法性判断不正确")
	}
}

func TestValidateProposalPayload(t *testing.T) {
	cases := []struct {
		name    string
		entity  ProposalEntity
		payload map[string]any
		wantErr bool
	}{
		{"人物有名字", EntityCharacter, map[string]any{"name": "林默"}, false},
		{"人物缺名字", EntityCharacter, map[string]any{}, true},
		{"人物名字为空串", EntityCharacter, map[string]any{"name": "   "}, true},
		{"事件有标题", EntityEvent, map[string]any{"title": "意外"}, false},
		{"事件缺标题", EntityEvent, map[string]any{}, true},
		{"章节摘要", EntityChapterSummary, map[string]any{"summary": "本章……"}, false},
		{"章节摘要为空", EntityChapterSummary, map[string]any{"summary": ""}, true},
		{"地点有名字", EntityLocation, map[string]any{"name": "青云城"}, false},
		{"势力有名字", EntityFaction, map[string]any{"name": "天枢阁"}, false},
		{"规则有名字", EntityWorldRule, map[string]any{"name": "灵力守恒"}, false},
		{"世界允许空名", EntityWorld, map[string]any{"description": "只更新描述"}, false},
		{"剧情弧类型合法", EntityPlotArc, map[string]any{"title": "主线", "type": "main"}, false},
		{"剧情弧类型非法", EntityPlotArc, map[string]any{"title": "主线", "type": "bogus"}, true},
		{"剧情弧缺标题", EntityPlotArc, map[string]any{"type": "main"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateProposalPayload(tc.entity, tc.payload)
			if tc.wantErr && !errors.Is(err, ErrProposalPayloadInvalid) {
				t.Fatalf("应返回 ErrProposalPayloadInvalid，实际 %v", err)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("应通过校验，实际 %v", err)
			}
		})
	}
}

func TestProposalValidateAndNormalize(t *testing.T) {
	p := &AnalysisProposal{
		WorkID:     "w1",
		Stage:      StageCharacterExtract,
		EntityType: EntityCharacter,
		Title:      "  林默 ",
		Payload:    map[string]any{"name": "林默"},
	}
	p.Normalize()
	if p.Title != "林默" {
		t.Errorf("标题应去空白，实际 %q", p.Title)
	}
	if p.Status != ProposalPending {
		t.Errorf("默认状态应为 PENDING，实际 %s", p.Status)
	}
	if err := p.Validate(); err != nil {
		t.Fatalf("应通过校验: %v", err)
	}

	bad := &AnalysisProposal{Stage: "bogus", EntityType: EntityCharacter, Payload: map[string]any{"name": "x"}}
	if err := bad.Validate(); !errors.Is(err, ErrProposalStageInvalid) {
		t.Errorf("非法阶段应报错，实际 %v", err)
	}
	badEntity := &AnalysisProposal{Stage: StageCharacterExtract, EntityType: "bogus", Payload: map[string]any{}}
	if err := badEntity.Validate(); !errors.Is(err, ErrProposalEntityInvalid) {
		t.Errorf("非法实体类型应报错，实际 %v", err)
	}
}

func TestProposalStatusValid(t *testing.T) {
	for _, s := range []ProposalStatus{ProposalPending, ProposalApproved, ProposalRejected} {
		if !s.Valid() {
			t.Errorf("%s 应合法", s)
		}
	}
	if ProposalStatus("MAYBE").Valid() {
		t.Error("未知状态不应合法")
	}
}
