package ai

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"reflect"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/yanaoyi/novamindv2/backend/prompts"
)

// JSON Schema 校验（规格书 §57 / Phase 9 §9.5）。
//
// 为什么手写而不引第三方库（任务书 §9.5 允许二选一）：
//   - 只需要一个很小的子集（object/array/string + required/enum/length/items）；
//   - 关键诉求是"schema 与校验器不许悄悄漂移"—— 因此**不认识的关键字直接报错**，
//     而不是像一般实现那样忽略未知关键字（忽略意味着写错的约束会静默失效）；
//   - 少一个依赖，部署与审计都更简单。
//
// 支持的子集（其余关键字会报"不支持的关键字"）：
//   type(object/array/string/integer/number/boolean)、properties、required、
//   additionalProperties(false 才会拒绝多余字段)、items、minItems、maxItems、
//   enum、minLength、maxLength、minimum、maximum。

// ErrSchemaNotFound 表示找不到对应的 schema 文件。
var ErrSchemaNotFound = errors.New("JSON Schema 不存在")

// SchemaError 是一次校验失败（问题清单面向模型：会被原样喂回去让它修一次）。
type SchemaError struct {
	Schema   string
	Problems []string
}

func (e *SchemaError) Error() string {
	return fmt.Sprintf("输出不符合 %s 的 JSON Schema：%s", e.Schema, strings.Join(e.Problems, "；"))
}

var (
	schemaCache  sync.Map // name -> map[string]any
	knownKeyword = map[string]bool{
		"type": true, "properties": true, "required": true, "additionalProperties": true,
		"items": true, "minItems": true, "maxItems": true,
		"enum": true, "minLength": true, "maxLength": true, "minimum": true, "maximum": true,
	}
)

// ValidateJSON 校验一段 JSON 文本是否符合内置 schema（name 对应 prompts/schemas/<name>.json）。
func ValidateJSON(data []byte, schemaName string) error {
	schema, err := loadSchema(schemaName)
	if err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var doc any
	if err := dec.Decode(&doc); err != nil {
		return fmt.Errorf("不是合法 JSON：%w", err)
	}
	// 禁止尾随内容：模型偶尔会"JSON 后面再补一段解释"，那种输出不能当合格结果
	if dec.More() {
		return fmt.Errorf("JSON 之后还有多余内容（模型不该在 JSON 外输出解释）")
	}
	return validateValue(schema, doc, "$", schemaName)
}

// ValidateObject 校验一个已经解析好的值（调用方通常已经用 ExtractJSONObject 容错解出 object）。
func ValidateObject(value any, schemaName string) error {
	schema, err := loadSchema(schemaName)
	if err != nil {
		return err
	}
	return validateValue(schema, value, "$", schemaName)
}

// loadSchema 读取并缓存 schema；同时校验 schema 自身只用到了受支持的关键字。
func loadSchema(name string) (map[string]any, error) {
	name = strings.TrimSpace(name)
	if v, ok := schemaCache.Load(name); ok {
		return v.(map[string]any), nil
	}
	raw, err := prompts.FS.ReadFile(path.Join("schemas", name+".json"))
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrSchemaNotFound, name)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var schema map[string]any
	if err := dec.Decode(&schema); err != nil {
		return nil, fmt.Errorf("解析 schema %s 失败: %w", name, err)
	}
	if err := checkKeywords(schema, "$"); err != nil {
		return nil, fmt.Errorf("schema %s 自身不合法: %w", name, err)
	}
	schemaCache.Store(name, schema)
	return schema, nil
}

// checkKeywords 检查 schema 树里只出现受支持的关键字（fail loud，避免约束静默失效）。
func checkKeywords(schema map[string]any, at string) error {
	keys := make([]string, 0, len(schema))
	for k := range schema {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if !knownKeyword[k] {
			return fmt.Errorf("%s 使用了不支持的关键字 %q（校验器不认识它，静默忽略等于约束失效）", at, k)
		}
	}
	if props, ok := schema["properties"].(map[string]any); ok {
		for name, sub := range props {
			subSchema, ok := sub.(map[string]any)
			if !ok {
				return fmt.Errorf("%s.properties.%s 不是对象", at, name)
			}
			if err := checkKeywords(subSchema, at+".properties."+name); err != nil {
				return err
			}
		}
	}
	if items, ok := schema["items"].(map[string]any); ok {
		if err := checkKeywords(items, at+".items"); err != nil {
			return err
		}
	}
	return nil
}

func validateValue(schema map[string]any, value any, at, schemaName string) error {
	var problems []string
	validate(schema, value, at, &problems)
	if len(problems) > 0 {
		return &SchemaError{Schema: schemaName, Problems: problems}
	}
	return nil
}

func validate(schema map[string]any, value any, at string, problems *[]string) {
	if want, ok := schema["type"].(string); ok && !typeMatches(want, value) {
		*problems = append(*problems, fmt.Sprintf("%s 应为 %s，实际是 %s", at, want, typeName(value)))
		return
	}
	if enumRaw, ok := schema["enum"].([]any); ok {
		matched := false
		for _, want := range enumRaw {
			if reflect.DeepEqual(want, value) {
				matched = true
				break
			}
		}
		if !matched {
			*problems = append(*problems, fmt.Sprintf("%s 只能是 %s 之一，实际 %v", at, enumList(enumRaw), value))
		}
	}

	switch v := value.(type) {
	case map[string]any:
		if required, ok := schema["required"].([]any); ok {
			for _, raw := range required {
				name, _ := raw.(string)
				if _, exists := v[name]; !exists {
					*problems = append(*problems, fmt.Sprintf("%s 缺少必填字段 %q", at, name))
				}
			}
		}
		props, _ := schema["properties"].(map[string]any)
		for name, sub := range props {
			child, exists := v[name]
			if !exists {
				continue
			}
			subSchema, ok := sub.(map[string]any)
			if !ok {
				continue
			}
			validate(subSchema, child, at+"."+name, problems)
		}
		if extra, ok := schema["additionalProperties"].(bool); ok && !extra {
			names := make([]string, 0, len(v))
			for name := range v {
				if _, known := props[name]; !known {
					names = append(names, name)
				}
			}
			if len(names) > 0 {
				sort.Strings(names)
				*problems = append(*problems,
					fmt.Sprintf("%s 出现了未定义字段 %s（该 schema 不允许额外字段）", at, strings.Join(names, ", ")))
			}
		}
	case []any:
		if min, ok := intKeyword(schema, "minItems"); ok && len(v) < min {
			*problems = append(*problems, fmt.Sprintf("%s 至少 %d 项，实际 %d 项", at, min, len(v)))
		}
		if max, ok := intKeyword(schema, "maxItems"); ok && len(v) > max {
			*problems = append(*problems, fmt.Sprintf("%s 最多 %d 项，实际 %d 项", at, max, len(v)))
		}
		if items, ok := schema["items"].(map[string]any); ok {
			for i, item := range v {
				validate(items, item, fmt.Sprintf("%s[%d]", at, i), problems)
			}
		}
	case string:
		n := utf8.RuneCountInString(v)
		if min, ok := intKeyword(schema, "minLength"); ok && n < min {
			*problems = append(*problems, fmt.Sprintf("%s 至少 %d 字，实际 %d 字", at, min, n))
		}
		if max, ok := intKeyword(schema, "maxLength"); ok && n > max {
			*problems = append(*problems, fmt.Sprintf("%s 最多 %d 字，实际 %d 字", at, max, n))
		}
	case json.Number:
		f, err := v.Float64()
		if err != nil {
			*problems = append(*problems, fmt.Sprintf("%s 不是合法数字: %v", at, v))
			return
		}
		checkRange(schema, f, at, problems)
	case float64:
		// json.Unmarshal（不启 UseNumber）解出来的数字走这里；约束必须照样生效
		checkRange(schema, v, at, problems)
	}
}

func checkRange(schema map[string]any, f float64, at string, problems *[]string) {
	if min, ok := floatKeyword(schema, "minimum"); ok && f < min {
		*problems = append(*problems, fmt.Sprintf("%s 不得小于 %v，实际 %v", at, min, f))
	}
	if max, ok := floatKeyword(schema, "maximum"); ok && f > max {
		*problems = append(*problems, fmt.Sprintf("%s 不得大于 %v，实际 %v", at, max, f))
	}
}

func typeMatches(want string, value any) bool {
	switch want {
	case "object":
		_, ok := value.(map[string]any)
		return ok
	case "array":
		_, ok := value.([]any)
		return ok
	case "string":
		_, ok := value.(string)
		return ok
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "integer", "number":
		switch num := value.(type) {
		case json.Number:
			if want == "number" {
				return true
			}
			_, err := num.Int64()
			return err == nil
		case float64:
			if want == "number" {
				return true
			}
			return num == float64(int64(num))
		default:
			return false
		}
	default:
		return false
	}
}

func typeName(value any) string {
	switch value.(type) {
	case nil:
		return "null"
	case map[string]any:
		return "对象"
	case []any:
		return "数组"
	case string:
		return "字符串"
	case bool:
		return "布尔值"
	case json.Number:
		return "数字"
	case float64:
		return "数字"
	default:
		return "未知类型"
	}
}

func enumList(values []any) string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		out = append(out, fmt.Sprintf("%v", v))
	}
	return strings.Join(out, " / ")
}

func intKeyword(schema map[string]any, key string) (int, bool) {
	num, ok := schema[key].(json.Number)
	if !ok {
		return 0, false
	}
	n, err := num.Int64()
	if err != nil {
		return 0, false
	}
	return int(n), true
}

func floatKeyword(schema map[string]any, key string) (float64, bool) {
	num, ok := schema[key].(json.Number)
	if !ok {
		return 0, false
	}
	f, err := num.Float64()
	if err != nil {
		return 0, false
	}
	return f, true
}
