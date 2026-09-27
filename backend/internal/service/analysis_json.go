package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// ErrModelJSONInvalid 表示模型输出无法解析成 JSON。
var ErrModelJSONInvalid = errors.New("模型输出不是合法 JSON")

// ExtractJSONObject 从模型输出里抽出第一个 JSON 对象。
//
// 现实情况：即使要求"只输出 JSON"，模型仍可能包上 ```json 代码块，
// 或前后带一句解释。这里做容错提取，避免整批任务因为这些噪声失败。
func ExtractJSONObject(content string) (map[string]any, error) {
	text := strings.TrimSpace(content)
	if text == "" {
		return nil, fmt.Errorf("%w：内容为空", ErrModelJSONInvalid)
	}

	// 去掉 ```json ... ``` 代码块包裹
	if strings.HasPrefix(text, "```") {
		if idx := strings.Index(text, "\n"); idx >= 0 {
			text = text[idx+1:]
		}
		text = strings.TrimSuffix(strings.TrimSpace(text), "```")
		text = strings.TrimSpace(text)
	}

	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start < 0 || end <= start {
		return nil, fmt.Errorf("%w：找不到完整的 JSON 对象", ErrModelJSONInvalid)
	}
	raw := text[start : end+1]

	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("%w：%v", ErrModelJSONInvalid, err)
	}
	return out, nil
}

// ExtractJSONArray 从模型输出里抽出第一段 JSON 数组。
func ExtractJSONArray(content string) ([]any, error) {
	text := strings.TrimSpace(content)
	if strings.HasPrefix(text, "```") {
		if idx := strings.Index(text, "\n"); idx >= 0 {
			text = text[idx+1:]
		}
		text = strings.TrimSuffix(strings.TrimSpace(text), "```")
	}
	start := strings.Index(text, "[")
	end := strings.LastIndex(text, "]")
	if start < 0 || end <= start {
		return nil, fmt.Errorf("%w：找不到完整的 JSON 数组", ErrModelJSONInvalid)
	}
	var out []any
	if err := json.Unmarshal([]byte(text[start:end+1]), &out); err != nil {
		return nil, fmt.Errorf("%w：%v", ErrModelJSONInvalid, err)
	}
	return out, nil
}

// IsJSONInvalid 判断错误是否为 JSON 解析类错误（用于决定是否触发"自动修复重试"）。
func IsJSONInvalid(err error) bool { return errors.Is(err, ErrModelJSONInvalid) }
