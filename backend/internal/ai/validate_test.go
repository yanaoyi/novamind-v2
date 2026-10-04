package ai

import (
	"errors"
	"testing"
)

func TestValidateAPIBaseBlocksInternalTargets(t *testing.T) {
	blocked := []string{
		"http://127.0.0.1:8080/v1",         // 环回
		"http://[::1]:8080/v1",             // IPv6 环回
		"http://10.0.0.5/v1",               // 私网
		"http://172.16.3.9/v1",             // 私网
		"http://192.168.1.10/v1",           // 私网
		"http://169.254.169.254/latest",    // 云元数据
		"http://0.0.0.0:8080/v1",           // 未指定地址
		"http://100.64.0.1/v1",             // CGNAT
		"http://localhost:11434/v1",        // 本机名（解析到环回）
		"ftp://api.example.com/v1",         // 非 http(s)
		"file:///etc/passwd",               // 非 http(s)
		"http://",                          // 缺主机名
		"http://does-not-exist.invalid/v1", // 解析失败
		"",                                 // 空
	}
	for _, raw := range blocked {
		if err := ValidateAPIBase(raw, false); !errors.Is(err, ErrAPIBaseNotAllowed) {
			t.Errorf("应拒绝 %q，实际 %v", raw, err)
		}
	}
}

func TestValidateAPIBaseAllowsPublicTargets(t *testing.T) {
	allowed := []string{
		"https://api.deepseek.com/v1", // 真实使用场景（公网域名）
		"http://93.184.216.34/v1",     // 公网字面量 IP
		"https://api.openai.com/v1",
	}
	for _, raw := range allowed {
		if err := ValidateAPIBase(raw, false); err != nil {
			t.Errorf("应允许 %q，实际 %v", raw, err)
		}
	}
}

func TestValidateAPIBasePrivateAllowedOnlyWhenOptedIn(t *testing.T) {
	// 本地冒烟要用 127.0.0.1 的假模型服务，必须显式打开开关（ALLOW_PRIVATE_MODEL_BASE=true）
	if err := ValidateAPIBase("http://127.0.0.1:19557/v1", true); err != nil {
		t.Errorf("显式允许内网时应放行，实际 %v", err)
	}
	if err := ValidateAPIBase("ftp://127.0.0.1/v1", true); err == nil {
		t.Error("即便允许内网，非 http(s) 也必须拒绝")
	}
}
