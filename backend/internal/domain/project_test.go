package domain

import (
	"errors"
	"strings"
	"testing"
)

func TestProjectValidate(t *testing.T) {
	cases := []struct {
		name    string
		project Project
		wantErr error
	}{
		{
			name:    "合法：二创文章",
			project: Project{Name: "人间真相", Type: ProjectTypeCreative},
		},
		{
			name:    "合法：原著文章 + 显式状态",
			project: Project{Name: "原著", Type: ProjectTypeOriginal, Status: ProjectStatusDraft},
		},
		{
			name:    "名称全为空白",
			project: Project{Name: "   ", Type: ProjectTypeOriginal},
			wantErr: ErrProjectNameRequired,
		},
		{
			name:    "名称超长（201 字）",
			project: Project{Name: strings.Repeat("字", 201), Type: ProjectTypeOriginal},
			wantErr: ErrProjectNameTooLong,
		},
		{
			name:    "类型缺失",
			project: Project{Name: "缺类型"},
			wantErr: ErrProjectTypeInvalid,
		},
		{
			name:    "类型非法",
			project: Project{Name: "坏类型", Type: "FANFIC"},
			wantErr: ErrProjectTypeInvalid,
		},
		{
			name:    "状态非法",
			project: Project{Name: "坏状态", Type: ProjectTypeOriginal, Status: "ZOMBIE"},
			wantErr: ErrProjectStatusBad,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.project.Validate()
			if tc.wantErr == nil {
				if err != nil {
					t.Fatalf("期望通过校验，实际报错: %v", err)
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("期望错误 %v，实际 %v", tc.wantErr, err)
			}
		})
	}
}

func TestProjectNormalize(t *testing.T) {
	p := Project{Name: "  人间真相  ", Description: "  描述  "}
	p.Normalize()

	if p.Name != "人间真相" {
		t.Errorf("名称未去除首尾空白: %q", p.Name)
	}
	if p.Description != "描述" {
		t.Errorf("描述未去除首尾空白: %q", p.Description)
	}
	if p.Status != ProjectStatusActive {
		t.Errorf("状态未补默认值，实际 %q", p.Status)
	}

	// 已有状态不被覆盖
	p2 := Project{Name: "x", Status: ProjectStatusArchived}
	p2.Normalize()
	if p2.Status != ProjectStatusArchived {
		t.Errorf("已有状态被覆盖: %q", p2.Status)
	}
}

func TestProjectTypeAndStatusValid(t *testing.T) {
	for _, ty := range []ProjectType{ProjectTypeOriginal, ProjectTypeCreative} {
		if !ty.Valid() {
			t.Errorf("%q 应合法", ty)
		}
	}
	if ProjectType("").Valid() {
		t.Error("空类型不应合法")
	}
	for _, s := range []ProjectStatus{ProjectStatusDraft, ProjectStatusActive, ProjectStatusArchived} {
		if !s.Valid() {
			t.Errorf("%q 应合法", s)
		}
	}
	if ProjectStatus("").Valid() {
		t.Error("空状态不应合法")
	}
}
