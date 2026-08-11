package response

import "time"

// WebSearchProviderResponse 网页搜索提供方响应（密钥脱敏，仅标记是否已配置）
type WebSearchProviderResponse struct {
	ID          uint                                `json:"id"`
	Name        string                              `json:"name"`
	Provider    string                              `json:"provider"`
	Description string                              `json:"description,omitempty"`
	Parameters  WebSearchProviderParametersResponse `json:"parameters"`
	IsDefault   bool                                `json:"is_default"`
	IsEnabled   bool                                `json:"is_enabled"`
	Credentials map[string]CredentialFieldMetadata  `json:"credentials"`
	CreatedAt   time.Time                           `json:"created_at"`
	UpdatedAt   time.Time                           `json:"updated_at"`
}

// WebSearchProviderParametersResponse 提供方参数响应（不含 api_key）
type WebSearchProviderParametersResponse struct {
	BaseURL     string            `json:"base_url,omitempty"`
	ProxyURL    string            `json:"proxy_url,omitempty"`
	ExtraConfig map[string]string `json:"extra_config,omitempty"`
}

// WebSearchProviderTypeResponse 引擎类型元数据（/types 接口，驱动前端动态表单）
type WebSearchProviderTypeResponse struct {
	ID                     string                         `json:"id"`
	Name                   string                         `json:"name"`
	RequiresAPIKey         bool                           `json:"requires_api_key"`
	SupportsOptionalAPIKey bool                           `json:"supports_optional_api_key,omitempty"`
	RequiresEngineID       bool                           `json:"requires_engine_id,omitempty"`
	RequiresBaseURL        bool                           `json:"requires_base_url,omitempty"`
	SupportsProxy          bool                           `json:"supports_proxy,omitempty"`
	Description            string                         `json:"description,omitempty"`
	DocsURL                string                         `json:"docs_url,omitempty"`
	ConfigFields           []WebSearchProviderConfigField `json:"config_fields,omitempty"`
}

// WebSearchProviderConfigField 类型附加配置字段（前端 t-select 动态渲染）
type WebSearchProviderConfigField struct {
	Key            string                          `json:"key"`
	Label          string                          `json:"label"`
	LabelKey       string                          `json:"label_key,omitempty"`
	Type           string                          `json:"type"`
	Required       bool                            `json:"required,omitempty"`
	Default        string                          `json:"default,omitempty"`
	Description    string                          `json:"description,omitempty"`
	DescriptionKey string                          `json:"description_key,omitempty"`
	Options        []WebSearchProviderConfigOption `json:"options,omitempty"`
}

// WebSearchProviderConfigOption 配置字段下拉选项
type WebSearchProviderConfigOption struct {
	Label    string `json:"label"`
	LabelKey string `json:"label_key,omitempty"`
	Value    string `json:"value"`
}

// WebSearchTestResultResponse 连通性测试响应
type WebSearchTestResultResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Count   int    `json:"count,omitempty"`
}

// WebSearchCredentialsResponse 凭据子资源响应（密钥本身不返回）
type WebSearchCredentialsResponse struct {
	Fields map[string]CredentialFieldMetadata `json:"fields"`
}
