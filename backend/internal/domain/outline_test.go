package domain

import (
	"errors"
	"testing"
)

func TestFlattenOutlineInputKeepsLevelsAndParents(t *testing.T) {
	inputs := []OutlineNodeInput{
		{
			Title: "第一卷", Summary: "卷概要",
			Children: []OutlineNodeInput{
				{
					Title: "第一节",
					Children: []OutlineNodeInput{
						{Title: "第一章", Purpose: "开场", Outcome: "钩子"},
						{Title: "第二章"},
					},
				},
				{Title: "第二节"},
			},
		},
		{Title: "第二卷"},
	}

	nodes, parents, err := FlattenOutlineInput(inputs)
	if err != nil {
		t.Fatalf("摊平失败: %v", err)
	}
	if len(nodes) != 6 {
		t.Fatalf("应有 6 个节点，实际 %d", len(nodes))
	}

	wantLevels := []OutlineLevel{
		OutlineLevelVolume, OutlineLevelSection, OutlineLevelChapter,
		OutlineLevelChapter, OutlineLevelSection, OutlineLevelVolume,
	}
	for i, want := range wantLevels {
		if nodes[i].Level != want {
			t.Errorf("第 %d 个节点层级应为 %d，实际 %d", i, want, nodes[i].Level)
		}
	}
	// 父子关系：第一章(2) → 第一节(1)，第二章(3) → 第一节(1)，第一节(1) → 第一卷(0)，第二卷(-1)
	wantParents := []int{-1, 0, 1, 1, 0, -1}
	for i, want := range wantParents {
		if parents[i] != want {
			t.Errorf("第 %d 个节点的父下标应为 %d，实际 %d", i, want, parents[i])
		}
	}
	if nodes[2].Purpose != "开场" || nodes[2].Outcome != "钩子" {
		t.Errorf("章节点的 purpose/outcome 未保留: %+v", nodes[2])
	}
}

func TestFlattenOutlineInputRejectsBadTrees(t *testing.T) {
	cases := []struct {
		name string
		in   []OutlineNodeInput
		want error
	}{
		{"空树", nil, ErrOutlineTreeEmpty},
		{"卷标题为空", []OutlineNodeInput{{Title: "   "}}, ErrOutlineNodeTitleEmpty},
		{
			"超过三层",
			[]OutlineNodeInput{{
				Title: "卷",
				Children: []OutlineNodeInput{{
					Title: "节",
					Children: []OutlineNodeInput{{
						Title:    "章",
						Children: []OutlineNodeInput{{Title: "再深一层"}},
					}},
				}},
			}},
			ErrOutlineTreeTooDeep,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := FlattenOutlineInput(tc.in); !errors.Is(err, tc.want) {
				t.Fatalf("期望错误 %v，实际 %v", tc.want, err)
			}
		})
	}
}

func TestBuildOutlineTreeSortsAndReparents(t *testing.T) {
	rootID := "root"
	firstID := "first"
	secondID := "second"

	nodes := []OutlineNode{
		// 故意乱序输入，且子节点先于父节点出现
		{ID: secondID, ParentID: &rootID, Level: OutlineLevelSection, Sequence: 2, Title: "第二节"},
		{ID: firstID, ParentID: &rootID, Level: OutlineLevelSection, Sequence: 1, Title: "第一节"},
		{ID: rootID, Level: OutlineLevelVolume, Sequence: 1, Title: "第一卷"},
		{ID: "orphan", ParentID: strPtr("missing"), Level: OutlineLevelSection, Sequence: 9, Title: "野节点"},
	}

	trees := BuildOutlineTree(nodes)
	if len(trees) != 2 {
		t.Fatalf("应有 2 个根节点（卷 + 被提升的野节点），实际 %d", len(trees))
	}
	root := trees[0]
	if root.Title != "第一卷" {
		t.Fatalf("根节点应为第一卷，实际 %s", root.Title)
	}
	if len(root.Children) != 2 {
		t.Fatalf("第一卷下应有 2 个节，实际 %d", len(root.Children))
	}
	if root.Children[0].Title != "第一节" || root.Children[1].Title != "第二节" {
		t.Errorf("同级未按 sequence 排序: %s, %s", root.Children[0].Title, root.Children[1].Title)
	}
	if trees[1].Title != "野节点" {
		t.Errorf("父节点缺失的节点应被提升为根，实际 %s", trees[1].Title)
	}
}

func TestOutlineNodeValidate(t *testing.T) {
	parent := "p"
	rootWithParent := OutlineNode{Title: "卷", Level: OutlineLevelVolume, ParentID: &parent}
	if err := rootWithParent.Validate(); err == nil {
		t.Error("卷节点带父节点应报错")
	}
	sectionWithoutParent := OutlineNode{Title: "节", Level: OutlineLevelSection}
	if err := sectionWithoutParent.Validate(); err == nil {
		t.Error("节节点没有父节点应报错")
	}
	badLevel := OutlineNode{Title: "第四层", Level: OutlineLevel(4), ParentID: &parent}
	if !errors.Is(badLevel.Validate(), ErrOutlineLevelInvalid) {
		t.Errorf("非法层级应报 ErrOutlineLevelInvalid，实际 %v", badLevel.Validate())
	}
}

func TestOutlineNormalizeAndValidate(t *testing.T) {
	o := &Outline{Title: "  二创大纲  "}
	o.Normalize()
	if o.Title != "二创大纲" || o.Version != 1 || o.Source != OutlineSourceManual {
		t.Fatalf("默认值不正确: %+v", o)
	}
	if err := o.Validate(); err != nil {
		t.Fatalf("合法大纲不应报错: %v", err)
	}

	empty := &Outline{Title: "   "}
	empty.Normalize()
	if !errors.Is(empty.Validate(), ErrOutlineTitleEmpty) {
		t.Errorf("空标题应报 ErrOutlineTitleEmpty，实际 %v", empty.Validate())
	}

	badSource := &Outline{Title: "x", Version: 1, Source: OutlineSource("HUMAN")}
	if !errors.Is(badSource.Validate(), ErrOutlineSourceInvalid) {
		t.Errorf("非法来源应报 ErrOutlineSourceInvalid，实际 %v", badSource.Validate())
	}
}

func strPtr(s string) *string { return &s }
