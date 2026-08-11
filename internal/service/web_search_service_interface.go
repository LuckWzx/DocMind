package service

import (
	"context"

	"docmind/internal/model/dto/request"
	dto "docmind/internal/model/dto/response"
	"docmind/internal/model/entity"
	"docmind/internal/service/websearch"
)

// WebSearchService 网页搜索业务：provider 管理（用户隔离）+ 引擎搜索
type WebSearchService interface {
	// ===== Provider 管理（严格按 user_id 隔离）=====
	List(userID uint) ([]*dto.WebSearchProviderResponse, error)
	GetByUser(userID, id uint) (*dto.WebSearchProviderResponse, error)
	Create(userID uint, req *request.CreateWebSearchProviderRequest) (*dto.WebSearchProviderResponse, error)
	Update(userID, id uint, req *request.UpdateWebSearchProviderRequest) (*dto.WebSearchProviderResponse, error)
	Delete(userID, id uint) error
	// TestProvider 测试已保存的提供方配置
	TestProvider(userID, id uint) (*dto.WebSearchTestResultResponse, error)
	// TestRaw 测试未保存的配置（不落库，创建/编辑弹窗内联测试）
	TestRaw(req *request.TestWebSearchProviderRequest) (*dto.WebSearchTestResultResponse, error)
	// UpdateCredentials 更新 api_key（密钥子资源，响应不返回密钥）
	UpdateCredentials(userID, id uint, apiKey string) (*dto.WebSearchCredentialsResponse, error)
	// DeleteCredential 清除 api_key
	DeleteCredential(userID, id uint) (*dto.WebSearchCredentialsResponse, error)

	// ===== 引擎类型元数据 =====
	ProviderTypes() []*dto.WebSearchProviderTypeResponse

	// ===== Agent 工具使用 =====
	// ResolveForAgent 解析用户可用提供方：显式 ID 优先，缺失/无效时兜底默认或首个启用（nil = 无可用）
	ResolveForAgent(userID, providerID uint) (*entity.WebSearchProvider, error)
	// Search 按提供方配置执行搜索（query 必填，maxResults<=0 用提供方默认 5）
	Search(ctx context.Context, provider *entity.WebSearchProvider, query string, maxResults int) ([]websearch.Result, error)
}
