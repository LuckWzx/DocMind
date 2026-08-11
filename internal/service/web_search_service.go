package service

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"docmind/internal/model/dto/request"
	dto "docmind/internal/model/dto/response"
	"docmind/internal/model/entity"
	"docmind/internal/repository"
	"docmind/internal/service/websearch"
	bizerrors "docmind/pkg/errors"
)

// webSearchTestTimeout 提供方连通性测试超时
const webSearchTestTimeout = 15 * time.Second

// webSearchService 网页搜索业务实现
type webSearchService struct {
	repo    repository.WebSearchProviderRepository
	factory *websearch.EngineFactory
}

// NewWebSearchService 创建网页搜索业务
func NewWebSearchService(repo repository.WebSearchProviderRepository, factory *websearch.EngineFactory) WebSearchService {
	return &webSearchService{repo: repo, factory: factory}
}

// ===== Provider 管理 =====

// List 获取当前用户全部提供方（严格隔离，不含内置）
func (s *webSearchService) List(userID uint) ([]*dto.WebSearchProviderResponse, error) {
	list, err := s.repo.ListByUser(userID)
	if err != nil {
		return nil, err
	}
	out := make([]*dto.WebSearchProviderResponse, 0, len(list))
	for _, p := range list {
		out = append(out, toWebSearchProviderResponse(p))
	}
	return out, nil
}

// GetByUser 获取当前用户指定提供方
func (s *webSearchService) GetByUser(userID, id uint) (*dto.WebSearchProviderResponse, error) {
	p, err := s.getOwned(userID, id)
	if err != nil {
		return nil, err
	}
	return toWebSearchProviderResponse(p), nil
}

// Create 创建提供方（用户隔离；首个提供方自动设为默认）
func (s *webSearchService) Create(userID uint, req *request.CreateWebSearchProviderRequest) (*dto.WebSearchProviderResponse, error) {
	provider := strings.ToLower(strings.TrimSpace(req.Provider))
	if _, err := s.factory.Create(provider); err != nil {
		return nil, bizerrors.New(bizerrors.CodeInvalidParam, err.Error())
	}
	if strings.TrimSpace(req.Name) == "" {
		return nil, bizerrors.New(bizerrors.CodeInvalidParam, "提供方名称不能为空")
	}

	existing, err := s.repo.ListByUser(userID)
	if err != nil {
		return nil, err
	}
	p := &entity.WebSearchProvider{
		UserID:      userID,
		Name:        strings.TrimSpace(req.Name),
		Provider:    provider,
		Description: req.Description,
		IsEnabled:   true,
		IsDefault:   len(existing) == 0 || req.IsDefault, // 首个提供方强制默认，其余按请求
	}
	applyParameters(p, req.Parameters)
	if p.IsDefault {
		if err := s.repo.ClearDefault(userID); err != nil {
			return nil, err
		}
	}
	if err := s.repo.Create(p); err != nil {
		return nil, err
	}
	return toWebSearchProviderResponse(p), nil
}

// Update 更新提供方（api_key 不在本接口更新，走 /credentials）
func (s *webSearchService) Update(userID, id uint, req *request.UpdateWebSearchProviderRequest) (*dto.WebSearchProviderResponse, error) {
	p, err := s.getOwned(userID, id)
	if err != nil {
		return nil, err
	}
	if req.Name != "" {
		p.Name = strings.TrimSpace(req.Name)
	}
	if req.Description != "" {
		p.Description = req.Description
	}
	if req.Parameters != nil {
		applyParameters(p, req.Parameters)
	}
	if req.IsDefault != nil && *req.IsDefault {
		if err := s.repo.ClearDefault(userID); err != nil {
			return nil, err
		}
		p.IsDefault = true
	} else if req.IsDefault != nil {
		p.IsDefault = false
	}
	if req.IsEnabled != nil {
		p.IsEnabled = *req.IsEnabled
	}
	if err := s.repo.Update(p); err != nil {
		return nil, err
	}
	return toWebSearchProviderResponse(p), nil
}

// Delete 删除提供方（仅限本人）
func (s *webSearchService) Delete(userID, id uint) error {
	p, err := s.getOwned(userID, id)
	if err != nil {
		return err
	}
	return s.repo.Delete(p.ID)
}

// TestProvider 测试已保存的提供方
func (s *webSearchService) TestProvider(userID, id uint) (*dto.WebSearchTestResultResponse, error) {
	p, err := s.getOwned(userID, id)
	if err != nil {
		return nil, err
	}
	return s.runTest(p)
}

// TestRaw 测试未保存的配置（不落库）
func (s *webSearchService) TestRaw(req *request.TestWebSearchProviderRequest) (*dto.WebSearchTestResultResponse, error) {
	if req == nil {
		return nil, bizerrors.New(bizerrors.CodeInvalidParam, "测试请求不能为空")
	}
	p := &entity.WebSearchProvider{Provider: strings.ToLower(strings.TrimSpace(req.Provider))}
	applyParameters(p, req.Parameters)
	return s.runTest(p)
}

// UpdateCredentials 更新 api_key（密钥子资源）
func (s *webSearchService) UpdateCredentials(userID, id uint, apiKey string) (*dto.WebSearchCredentialsResponse, error) {
	p, err := s.getOwned(userID, id)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(apiKey) == "" {
		return nil, bizerrors.New(bizerrors.CodeInvalidParam, "api_key 不能为空")
	}
	p.APIKey = strings.TrimSpace(apiKey)
	if err := s.repo.Update(p); err != nil {
		return nil, err
	}
	return webSearchCredentialsResponse(p), nil
}

// DeleteCredential 清除 api_key
func (s *webSearchService) DeleteCredential(userID, id uint) (*dto.WebSearchCredentialsResponse, error) {
	p, err := s.getOwned(userID, id)
	if err != nil {
		return nil, err
	}
	p.APIKey = ""
	if err := s.repo.Update(p); err != nil {
		return nil, err
	}
	return webSearchCredentialsResponse(p), nil
}

// ===== 引擎类型元数据 =====

// ProviderTypes 引擎类型静态元数据（前端动态表单驱动）
func (s *webSearchService) ProviderTypes() []*dto.WebSearchProviderTypeResponse {
	return []*dto.WebSearchProviderTypeResponse{
		{
			ID:             websearch.EngineDuckDuckGo,
			Name:           "DuckDuckGo",
			RequiresAPIKey: false,
			SupportsProxy:  true,
			Description:    "免费无需 API Key，海外服务国内网络可能不可达，可配置代理",
			DocsURL:        "https://duckduckgo.com",
		},
		{
			ID:             websearch.EngineTavily,
			Name:           "Tavily",
			RequiresAPIKey: true,
			SupportsProxy:  true,
			Description:    "官方搜索 API，结果质量稳定，需注册获取 API Key",
			DocsURL:        "https://docs.tavily.com",
		},
		{
			ID:             websearch.EngineBaidu,
			Name:           "百度搜索",
			RequiresAPIKey: true,
			Description:    "千帆「百度搜索」API，需千帆 API Key（bce-v3/ALTAK 格式），每月 1500 次免费额度",
			DocsURL:        "https://cloud.baidu.com/doc/qianfan-api/s/Wmbq4z7e5",
		},
	}
}

// ===== Agent 工具使用 =====

// ResolveForAgent 解析用户可用提供方：显式 ID 优先（不存在/未启用时兜底），
// 未指定时取默认提供方，无默认取首个启用；均无返回 nil
func (s *webSearchService) ResolveForAgent(userID, providerID uint) (*entity.WebSearchProvider, error) {
	if providerID > 0 {
		p, err := s.repo.FindByUserAndID(userID, providerID)
		if err != nil {
			return nil, err
		}
		if p != nil && p.IsEnabled {
			return p, nil
		}
	}
	list, err := s.repo.ListEnabledByUser(userID)
	if err != nil {
		return nil, err
	}
	for _, p := range list {
		if p.IsDefault {
			return p, nil
		}
	}
	if len(list) > 0 {
		return list[0], nil
	}
	return nil, nil
}

// Search 按提供方配置执行搜索
func (s *webSearchService) Search(ctx context.Context, provider *entity.WebSearchProvider, query string, maxResults int) ([]websearch.Result, error) {
	engine, err := s.factory.Create(provider.Provider)
	if err != nil {
		return nil, err
	}
	return engine.Search(ctx, query, s.toSearchOptions(provider, maxResults))
}

// ===== 内部辅助 =====

// getOwned 按归属校验获取提供方
func (s *webSearchService) getOwned(userID, id uint) (*entity.WebSearchProvider, error) {
	p, err := s.repo.FindByUserAndID(userID, id)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, bizerrors.ErrResourceNotFound
	}
	return p, nil
}

// runTest 执行连通性测试
func (s *webSearchService) runTest(p *entity.WebSearchProvider) (*dto.WebSearchTestResultResponse, error) {
	engine, err := s.factory.Create(p.Provider)
	if err != nil {
		return &dto.WebSearchTestResultResponse{Success: false, Message: err.Error()}, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), webSearchTestTimeout)
	defer cancel()
	if err := engine.Test(ctx, s.toSearchOptions(p, 1)); err != nil {
		return &dto.WebSearchTestResultResponse{Success: false, Message: err.Error()}, nil
	}
	return &dto.WebSearchTestResultResponse{Success: true, Message: "连接成功"}, nil
}

// toSearchOptions 实体 → 引擎参数
func (s *webSearchService) toSearchOptions(p *entity.WebSearchProvider, maxResults int) websearch.SearchOptions {
	opts := websearch.SearchOptions{
		MaxResults: maxResults,
		APIKey:     p.APIKey,
		BaseURL:    p.BaseURL,
		ProxyURL:   p.ProxyURL,
	}
	if len(p.ExtraConfig) > 0 {
		var extra map[string]string
		if err := json.Unmarshal(p.ExtraConfig, &extra); err == nil {
			opts.Extra = extra
		}
	}
	return opts
}

// applyParameters 请求参数 → 实体（api_key 仅在创建/测试时写入）
func applyParameters(p *entity.WebSearchProvider, params *request.WebSearchProviderParameters) {
	if params == nil {
		return
	}
	if params.APIKey != "" {
		p.APIKey = strings.TrimSpace(params.APIKey)
	}
	p.BaseURL = strings.TrimSpace(params.BaseURL)
	p.ProxyURL = strings.TrimSpace(params.ProxyURL)
	if len(params.ExtraConfig) > 0 {
		p.ExtraConfig = marshalJSONOrNil(params.ExtraConfig)
	}
}

// toWebSearchProviderResponse 实体 → 响应（api_key 脱敏，仅标记已配置）
func toWebSearchProviderResponse(p *entity.WebSearchProvider) *dto.WebSearchProviderResponse {
	resp := &dto.WebSearchProviderResponse{
		ID:          p.ID,
		Name:        p.Name,
		Provider:    p.Provider,
		Description: p.Description,
		Parameters: dto.WebSearchProviderParametersResponse{
			BaseURL:  p.BaseURL,
			ProxyURL: p.ProxyURL,
		},
		IsDefault: p.IsDefault,
		IsEnabled: p.IsEnabled,
		Credentials: map[string]dto.CredentialFieldMetadata{
			"api_key": {Configured: p.APIKey != ""},
		},
		CreatedAt: p.CreatedAt,
		UpdatedAt: p.UpdatedAt,
	}
	if len(p.ExtraConfig) > 0 {
		var extra map[string]string
		if err := json.Unmarshal(p.ExtraConfig, &extra); err == nil && len(extra) > 0 {
			resp.Parameters.ExtraConfig = extra
		}
	}
	return resp
}

// webSearchCredentialsResponse 凭据子资源响应
func webSearchCredentialsResponse(p *entity.WebSearchProvider) *dto.WebSearchCredentialsResponse {
	return &dto.WebSearchCredentialsResponse{
		Fields: map[string]dto.CredentialFieldMetadata{
			"api_key": {Configured: p.APIKey != ""},
		},
	}
}
