package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// 模型配置相关错误。
var (
	ErrProviderNameEmpty   = errors.New("模型配置名称不能为空")
	ErrProviderTypeInvalid = errors.New("模型提供商类型非法")
	ErrProviderBaseEmpty   = errors.New("接口地址不能为空")
	ErrProviderModelEmpty  = errors.New("模型名不能为空")
	ErrProviderTempRange   = errors.New("温度必须在 0-2 之间")
	ErrProviderNotFound    = errors.New("模型配置不存在")
	ErrProviderDuplicate   = errors.New("同名模型配置已存在")
	// ErrProviderDefaultExists：同一用途只能有一个默认配置（唯一索引 uq_model_providers_default）。
	// 单独成一个错误，是为了不让它被当成"同名冲突"报出去 —— 那条提示会把人带向错误方向。
	ErrProviderDefaultExists = errors.New("该用途已经有默认模型配置了，请先取消原默认，或直接修改原配置")
	ErrProviderPurposeBad    = errors.New("模型用途非法")
	ErrNoProviderAvailable   = errors.New("没有可用的模型配置")
)

// ProviderType 是模型提供商类型。
//
// OPENAI_COMPATIBLE 覆盖 OpenAI / DeepSeek / 智谱 / Kimi / one-api / vLLM 等
// 所有兼容 /chat/completions 的服务；ANTHROPIC 走 Messages API。
// 业务层只认这个枚举，不认任何厂商 SDK（规格书 §36）。
type ProviderType string

const (
	ProviderOpenAICompatible ProviderType = "OPENAI_COMPATIBLE"
	ProviderAnthropic        ProviderType = "ANTHROPIC"
)

// Valid 判断提供商类型是否合法。
func (t ProviderType) Valid() bool {
	return t == ProviderOpenAICompatible || t == ProviderAnthropic
}

// ProviderPurpose 是模型用途。
type ProviderPurpose string

const (
	PurposeChat      ProviderPurpose = "chat"
	PurposeEmbedding ProviderPurpose = "embedding"
	PurposeBoth      ProviderPurpose = "both"
)

// Valid 判断用途是否合法。
func (p ProviderPurpose) Valid() bool {
	return p == PurposeChat || p == PurposeEmbedding || p == PurposeBoth
}

// ModelProvider 是模型接入配置。
//
// 注意：结构体里**没有明文密钥字段** —— 密钥只在 service 层解密后直接交给
// gateway 使用，不进入任何返回值或日志（规格书 §64.13）。
type ModelProvider struct {
	ID        string
	Name      string
	Provider  ProviderType
	APIBase   string
	ModelName string
	// EmbedAPIBase / EmbedModelName 是向量模型的独立配置（Phase 9 §9.1.3）：
	// 为空时按约定回退（embed_api_base → api_base；embed_model_name → DEFAULT_EMBED_MODEL）。
	EmbedAPIBase   string
	EmbedModelName string
	Purpose        ProviderPurpose
	Temperature    float64
	MaxTokens      int
	TimeoutSec     int
	Enabled        bool
	IsDefault      bool
	Notes          string
	// HasAPIKey 表示库里是否已存密钥（API 只暴露这个布尔值）
	HasAPIKey bool
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

// Normalize 清洗输入并补默认值。
func (m *ModelProvider) Normalize() {
	m.Name = strings.TrimSpace(m.Name)
	m.APIBase = strings.TrimRight(strings.TrimSpace(m.APIBase), "/")
	m.ModelName = strings.TrimSpace(m.ModelName)
	m.EmbedAPIBase = strings.TrimRight(strings.TrimSpace(m.EmbedAPIBase), "/")
	m.EmbedModelName = strings.TrimSpace(m.EmbedModelName)
	m.Notes = strings.TrimSpace(m.Notes)
	if m.Provider == "" {
		m.Provider = ProviderOpenAICompatible
	}
	if m.Purpose == "" {
		m.Purpose = PurposeChat
	}
	if m.Temperature == 0 {
		m.Temperature = 0.7
	}
	if m.MaxTokens == 0 {
		m.MaxTokens = 4096
	}
	if m.TimeoutSec == 0 {
		m.TimeoutSec = 120
	}
}

// Validate 校验模型配置。
func (m *ModelProvider) Validate() error {
	if strings.TrimSpace(m.Name) == "" {
		return ErrProviderNameEmpty
	}
	if !m.Provider.Valid() {
		return fmt.Errorf("%w: %s", ErrProviderTypeInvalid, m.Provider)
	}
	if strings.TrimSpace(m.APIBase) == "" {
		return ErrProviderBaseEmpty
	}
	if strings.TrimSpace(m.ModelName) == "" {
		return ErrProviderModelEmpty
	}
	if !m.Purpose.Valid() {
		return fmt.Errorf("%w: %s", ErrProviderPurposeBad, m.Purpose)
	}
	if m.Temperature < 0 || m.Temperature > 2 {
		return ErrProviderTempRange
	}
	return nil
}
