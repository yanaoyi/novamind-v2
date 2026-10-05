package ai

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/yanaoyi/novamindv2/backend/internal/domain"
)

func TestValidateJSONAcceptsWellFormedFactExtract(t *testing.T) {
	doc := `{
	  "facts": [
	    {"kind": "character_state", "subject": "沈砚", "fact": "左臂受伤，无法用剑"},
	    {"kind": "item", "subject": "青铜钥匙", "fact": "藏在祖宅第三块砖下"}
	  ],
	  "summary": "沈砚回到老宅，取走青铜钥匙。"
	}`
	if err := ValidateJSON([]byte(doc), "fact_extract"); err != nil {
		t.Fatalf("合法输出不该报错: %v", err)
	}
}

func TestValidateJSONReportsActionableProblems(t *testing.T) {
	cases := []struct {
		name string
		doc  string
		want string
	}{
		{
			name: "缺少必填字段",
			doc:  `{"facts": [{"kind": "item", "subject": "钥匙"}], "summary": "x"}`,
			want: "$.facts[0] 缺少必填字段 \"fact\"",
		},
		{
			name: "枚举越界（模型最容易犯的错）",
			doc:  `{"facts": [{"kind": "mood", "subject": "沈砚", "fact": "心情好"}], "summary": "x"}`,
			want: "character_state",
		},
		{
			name: "额外字段",
			doc:  `{"facts": [], "summary": "x", "confidence": 0.9}`,
			want: "未定义字段 confidence",
		},
		{
			name: "长度为 0",
			doc:  `{"facts": [], "summary": ""}`,
			want: "$.summary 至少 1 字",
		},
		{
			name: "facts 不是数组",
			doc:  `{"facts": "无", "summary": "x"}`,
			want: "$.facts 应为 array",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateJSON([]byte(c.doc), "fact_extract")
			if err == nil {
				t.Fatal("应报错")
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("错误信息应能让模型直接照着改（含 %q），实际: %v", c.want, err)
			}
			var schemaErr *SchemaError
			if !errors.As(err, &schemaErr) {
				t.Errorf("应是 *SchemaError（便于上层区分「输出不合法」与「系统故障」），实际 %T", err)
			}
		})
	}
}

func TestValidateJSONRejectsNonJSONAndTrailingContent(t *testing.T) {
	if err := ValidateJSON([]byte("```json\n{}\n```"), "fact_extract"); err == nil {
		t.Error("带代码块标记的文本不是合法 JSON，应报错（调用方应先做容错提取）")
	}
	if err := ValidateJSON([]byte(`{"facts": [], "summary": "x"} 以上就是全部`), "fact_extract"); err == nil {
		t.Error("JSON 之后还有解释文字时应报错")
	}
}

func TestValidateObjectAcceptsParsedMap(t *testing.T) {
	var doc map[string]any
	if err := json.Unmarshal([]byte(`{"facts": [], "summary": "沈砚回到老宅。"}`), &doc); err != nil {
		t.Fatal(err)
	}
	// 注意：这里走的是 json.Unmarshal 的 float64 路径，schema 校验必须照样工作
	if err := ValidateObject(doc, "fact_extract"); err != nil {
		t.Fatalf("已解析对象应校验通过: %v", err)
	}
}

func TestValidateUnknownSchemaName(t *testing.T) {
	if err := ValidateJSON([]byte(`{}`), "no_such_schema"); !errors.Is(err, ErrSchemaNotFound) {
		t.Errorf("未知 schema 应返回 ErrSchemaNotFound，实际 %v", err)
	}
}

// schema 里的 kind 枚举必须与领域常量一致 —— 两处各写一份迟早漂移，
// 而那会让"新增一种事实类型却永远存不进去"这种问题静默发生。
func TestFactExtractSchemaKindEnumMatchesDomain(t *testing.T) {
	schema, err := loadSchema("fact_extract")
	if err != nil {
		t.Fatalf("读取 schema 失败: %v", err)
	}
	props := schema["properties"].(map[string]any)
	items := props["facts"].(map[string]any)["items"].(map[string]any)
	kinds := items["properties"].(map[string]any)["kind"].(map[string]any)["enum"].([]any)
	if len(kinds) != len(domain.FactKinds()) {
		t.Fatalf("schema 枚举 %d 项，领域常量 %d 项", len(kinds), len(domain.FactKinds()))
	}
	for i, k := range domain.FactKinds() {
		if kinds[i] != k {
			t.Errorf("第 %d 项不一致：schema=%v domain=%s", i, kinds[i], k)
		}
	}
}

// 校验器不认识的关键字必须报错，而不是静默忽略（静默忽略等于约束失效）。
func TestCheckKeywordsFailsLoudOnUnsupportedKeyword(t *testing.T) {
	bad := map[string]any{"type": "object", "oneOf": []any{map[string]any{"type": "string"}}}
	err := checkKeywords(bad, "$")
	if err == nil || !strings.Contains(err.Error(), "oneOf") {
		t.Errorf("不支持的关键字应报错并点名，实际 %v", err)
	}
}
