package service

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// fakeRunner 按调用次数返回预设文本，用来验证"截断 → 收敛重试"的行为。
type fakeRunner struct {
	replies []string
	calls   []map[string]any
}

func (f *fakeRunner) RunPrompt(_ context.Context, _ string, data any) (string, error) {
	vars, _ := data.(map[string]any)
	if vars == nil {
		vars = map[string]any{}
	}
	f.calls = append(f.calls, vars)
	if len(f.calls) > len(f.replies) {
		return "", errors.New("没有更多预设回复")
	}
	return f.replies[len(f.calls)-1], nil
}

func (f *fakeRunner) RunTextPrompt(ctx context.Context, name string, data any) (string, error) {
	return f.RunPrompt(ctx, name, data)
}

func TestRunJSONPromptRetriesWithConciseHintOnTruncatedJSON(t *testing.T) {
	runner := &fakeRunner{replies: []string{
		`{"volumes": [{"title": "第一卷", "sections": [{"title": "第一节"`, // 被截断
		`{"volumes": [{"title": "第一卷", "sections": [{"title": "第一节", "chapters": [{"title": "第一章"}]}]}]}`,
	}}

	out, err := RunJSONPrompt(context.Background(), runner, "outline_generate", map[string]any{"Instruction": "三卷结构"}, 2)
	if err != nil {
		t.Fatalf("重试后应当成功: %v", err)
	}
	if _, ok := out["volumes"]; !ok {
		t.Fatalf("解析结果缺少 volumes: %+v", out)
	}
	if len(runner.calls) != 2 {
		t.Fatalf("应当调用 2 次（失败一次 + 重试一次），实际 %d", len(runner.calls))
	}
	second, _ := runner.calls[1]["Instruction"].(string)
	if !strings.Contains(second, "更简洁") || !strings.Contains(second, "三卷结构") {
		t.Errorf("重试提示应保留原要求并追加收敛说明，实际: %s", second)
	}
	first, _ := runner.calls[0]["Instruction"].(string)
	if strings.Contains(first, "更简洁") {
		t.Error("首次调用不应带上收敛提示")
	}
}

func TestRunJSONPromptFailsAfterAllAttempts(t *testing.T) {
	runner := &fakeRunner{replies: []string{`not json`, `still not json`}}
	_, err := RunJSONPrompt(context.Background(), runner, "outline_generate", map[string]any{}, 2)
	if err == nil {
		t.Fatal("两次都失败时应返回错误")
	}
	if !strings.Contains(err.Error(), "已重试 2 次") {
		t.Errorf("错误里应说明重试次数，实际: %v", err)
	}
}
