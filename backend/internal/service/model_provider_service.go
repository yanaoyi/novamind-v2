package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/yanaoyi/novamindv2/backend/internal/ai"
	"github.com/yanaoyi/novamindv2/backend/internal/domain"
)

// ModelProviderRepository 是模型配置 service 需要的仓储能力。
type ModelProviderRepository interface {
	Create(ctx context.Context, p *domain.ModelProvider, apiKeyCipher string) error
	GetByID(ctx context.Context, id string) (*domain.ModelProvider, error)
	GetKeyCipher(ctx context.Context, id string) (string, error)
	List(ctx context.Context) ([]domain.ModelProvider, error)
	FindChatProvider(ctx context.Context) (*domain.ModelProvider, error)
	Update(ctx context.Context, p *domain.ModelProvider, apiKeyCipher *string) error
	SetDefault(ctx context.Context, id string, purpose domain.ProviderPurpose) error
	Delete(ctx context.Context, id string) error
}

// ModelProviderService 负责模型配置的增删改查与连通性测试。
//
// 密钥处理约定：
//   - 新增/更新时用 NOVAMIND_SECRET 加密后入库；
//   - 只有真正调用模型时才解密，且只存在于内存；
//   - 任何返回值都不含明文或密文（只暴露 HasAPIKey）。
type ModelProviderService struct {
	repo    ModelProviderRepository
	gateway *ai.Gateway
	secret  string
	// allowPrivateAPIBase 允许 api_base 指向内网（仅本地开发/冒烟需要，见 config.AllowPrivateModelBase）
	allowPrivateAPIBase bool
}

// NewModelProviderService 构建服务。
func NewModelProviderService(
	repo ModelProviderRepository,
	gateway *ai.Gateway,
	secret string,
	allowPrivateAPIBase bool,
) *ModelProviderService {
	return &ModelProviderService{repo: repo, gateway: gateway, secret: secret, allowPrivateAPIBase: allowPrivateAPIBase}
}

// ProviderInput 是模型配置入参。
type ProviderInput struct {
	Name        string
	Provider    domain.ProviderType
	APIBase     string
	APIKey      string // 明文；为空表示不设置/不修改
	ModelName   string
	Purpose     domain.ProviderPurpose
	Temperature float64
	MaxTokens   int
	TimeoutSec  int
	Enabled     bool
	IsDefault   bool
	Notes       string
}

// Create 新增模型配置。
func (s *ModelProviderService) Create(ctx context.Context, in ProviderInput) (*domain.ModelProvider, error) {
	if strings.TrimSpace(in.APIKey) != "" && strings.TrimSpace(s.secret) == "" {
		return nil, ai.ErrMissingSecret
	}
	cipher, err := ai.EncryptSecret(s.secret, in.APIKey)
	if err != nil && strings.TrimSpace(in.APIKey) != "" {
		return nil, err
	}

	p := &domain.ModelProvider{
		Name: in.Name, Provider: in.Provider, APIBase: in.APIBase, ModelName: in.ModelName,
		Purpose: in.Purpose, Temperature: in.Temperature, MaxTokens: in.MaxTokens,
		TimeoutSec: in.TimeoutSec, Enabled: in.Enabled, IsDefault: in.IsDefault, Notes: in.Notes,
	}
	p.Normalize()
	if err := p.Validate(); err != nil {
		return nil, err
	}
	// SSRF 收敛：api_base 会被服务端拿着已入库的真实 Key 去请求，必须先过地址校验
	if err := ai.ValidateAPIBase(p.APIBase, s.allowPrivateAPIBase); err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, p, cipher); err != nil {
		return nil, err
	}
	if p.IsDefault {
		if err := s.repo.SetDefault(ctx, p.ID, p.Purpose); err != nil {
			return nil, err
		}
	}
	return p, nil
}

// Update 更新模型配置；APIKey 为空表示保持原样。
func (s *ModelProviderService) Update(ctx context.Context, id string, in ProviderInput) (*domain.ModelProvider, error) {
	p, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	p.Name = in.Name
	p.Provider = in.Provider
	p.APIBase = in.APIBase
	p.ModelName = in.ModelName
	p.Purpose = in.Purpose
	p.Temperature = in.Temperature
	p.MaxTokens = in.MaxTokens
	p.TimeoutSec = in.TimeoutSec
	p.Enabled = in.Enabled
	p.IsDefault = in.IsDefault
	p.Notes = in.Notes
	p.Normalize()
	if err := p.Validate(); err != nil {
		return nil, err
	}
	if err := ai.ValidateAPIBase(p.APIBase, s.allowPrivateAPIBase); err != nil {
		return nil, err
	}

	var cipherPtr *string
	if strings.TrimSpace(in.APIKey) != "" {
		cipher, err := ai.EncryptSecret(s.secret, in.APIKey)
		if err != nil {
			return nil, err
		}
		cipherPtr = &cipher
	}
	if err := s.repo.Update(ctx, p, cipherPtr); err != nil {
		return nil, err
	}
	if p.IsDefault {
		if err := s.repo.SetDefault(ctx, p.ID, p.Purpose); err != nil {
			return nil, err
		}
	}
	return s.repo.GetByID(ctx, id)
}

// List 列出模型配置。
func (s *ModelProviderService) List(ctx context.Context) ([]domain.ModelProvider, error) {
	items, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []domain.ModelProvider{}
	}
	return items, nil
}

// Get 取模型配置。
func (s *ModelProviderService) Get(ctx context.Context, id string) (*domain.ModelProvider, error) {
	return s.repo.GetByID(ctx, id)
}

// SetDefault 设为默认。
func (s *ModelProviderService) SetDefault(ctx context.Context, id string) (*domain.ModelProvider, error) {
	p, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.repo.SetDefault(ctx, id, p.Purpose); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

// Delete 删除模型配置。
func (s *ModelProviderService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

// TestResult 是连通性测试结果。
type TestResult struct {
	OK           bool
	Model        string
	Reply        string
	LatencyMS    int64
	TotalTokens  int
	ErrorMessage string
}

// Test 用一条最小请求验证配置是否可用（会真实调用上游，消耗少量 token）。
func (s *ModelProviderService) Test(ctx context.Context, id string) (*TestResult, error) {
	cfg, err := s.ResolveConfig(ctx, id)
	if err != nil {
		return nil, err
	}
	p, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	resp, err := s.gateway.Chat(ctx, cfg, ai.ChatRequest{
		Messages: []ai.Message{
			{Role: "system", Content: "你是连通性测试助手，只回一个字。"},
			{Role: "user", Content: "请回复：好"},
		},
		Temperature: 0,
		MaxTokens:   16,
		TimeoutSec:  p.TimeoutSec,
	})
	if err != nil {
		// 完整错误写服务端日志，对外只给脱敏短消息（不回显上游响应体，避免内网探测 oracle）
		slog.Warn("模型连通性测试失败", slog.String("provider_id", id), slog.Any("error", err))
		return &TestResult{OK: false, ErrorMessage: ai.SanitizeError(err)}, nil
	}
	return &TestResult{
		OK: true, Model: resp.Model, Reply: strings.TrimSpace(resp.Content),
		LatencyMS: resp.LatencyMS, TotalTokens: resp.TotalTokens,
	}, nil
}

// ResolveConfig 解析出可直接调用的（已解密的）配置。
// id 为空时使用默认的对话模型。
func (s *ModelProviderService) ResolveConfig(ctx context.Context, id string) (ai.ProviderConfig, error) {
	var provider *domain.ModelProvider
	var err error
	if strings.TrimSpace(id) == "" {
		provider, err = s.repo.FindChatProvider(ctx)
	} else {
		provider, err = s.repo.GetByID(ctx, id)
	}
	if err != nil {
		return ai.ProviderConfig{}, err
	}
	if !provider.Enabled {
		return ai.ProviderConfig{}, fmt.Errorf("模型配置「%s」已停用", provider.Name)
	}

	cipher, err := s.repo.GetKeyCipher(ctx, provider.ID)
	if err != nil {
		return ai.ProviderConfig{}, err
	}
	key, err := ai.DecryptSecret(s.secret, cipher)
	if err != nil {
		return ai.ProviderConfig{}, err
	}
	if strings.TrimSpace(key) == "" {
		return ai.ProviderConfig{}, errors.New("该模型配置还没有填 API Key")
	}

	return ai.ProviderConfig{
		Type: provider.Provider, APIBase: provider.APIBase, APIKey: key, ModelName: provider.ModelName,
	}, nil
}

// ResolveRuntime 返回解析后的配置与原始参数（调用模型时带上温度/上限/超时）。
func (s *ModelProviderService) ResolveRuntime(ctx context.Context, id string) (ai.ProviderConfig, domain.ModelProvider, error) {
	cfg, err := s.ResolveConfig(ctx, id)
	if err != nil {
		return ai.ProviderConfig{}, domain.ModelProvider{}, err
	}
	var provider *domain.ModelProvider
	if strings.TrimSpace(id) == "" {
		provider, err = s.repo.FindChatProvider(ctx)
	} else {
		provider, err = s.repo.GetByID(ctx, id)
	}
	if err != nil {
		return ai.ProviderConfig{}, domain.ModelProvider{}, err
	}
	return cfg, *provider, nil
}
