package request

// WebSearchProviderParameters 提供方连接参数（api_key 仅在创建/测试时接收，响应不返回）
type WebSearchProviderParameters struct {
	APIKey      string            `json:"api_key,omitempty"`
	BaseURL     string            `json:"base_url,omitempty"`
	ProxyURL    string            `json:"proxy_url,omitempty"`
	EngineID    string            `json:"engine_id,omitempty"`
	ExtraConfig map[string]string `json:"extra_config,omitempty"`
}

// CreateWebSearchProviderRequest 创建网页搜索提供方
type CreateWebSearchProviderRequest struct {
	Name        string                       `json:"name"`
	Provider    string                       `json:"provider"`
	Description string                       `json:"description,omitempty"`
	Parameters  *WebSearchProviderParameters `json:"parameters,omitempty"`
	IsDefault   bool                         `json:"is_default,omitempty"`
}

// UpdateWebSearchProviderRequest 更新网页搜索提供方
// 注意：api_key 走 /credentials 子资源独立管理，本请求不接收
type UpdateWebSearchProviderRequest struct {
	Name        string                       `json:"name,omitempty"`
	Description string                       `json:"description,omitempty"`
	Parameters  *WebSearchProviderParameters `json:"parameters,omitempty"`
	IsDefault   *bool                        `json:"is_default,omitempty"`
	IsEnabled   *bool                        `json:"is_enabled,omitempty"`
}

// TestWebSearchProviderRequest 未保存配置的连通性测试（不落库）
type TestWebSearchProviderRequest struct {
	Provider   string                       `json:"provider"`
	Parameters *WebSearchProviderParameters `json:"parameters,omitempty"`
}

// UpdateWebSearchCredentialsRequest 更新提供方凭据（api_key 独立子资源）
type UpdateWebSearchCredentialsRequest struct {
	APIKey *string `json:"api_key,omitempty"`
}
