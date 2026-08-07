package request

// ModelParametersRequest 模型参数请求
type ModelParametersRequest struct {
	BaseURL             string                      `json:"base_url"`
	APIKey              string                      `json:"api_key"`
	AppID               string                      `json:"app_id"`
	AppSecret           string                      `json:"app_secret"`
	APIVersion          string                      `json:"api_version"`
	ModelName           string                      `json:"model_name"`
	Provider            string                      `json:"provider"`
	InterfaceType       string                      `json:"interface_type"`
	ParameterSize       string                      `json:"parameter_size"`
	Temperature         float64                     `json:"temperature"`
	MaxTokens           int                         `json:"max_tokens"`
	ContextWindow       int                         `json:"context_window"`
	Dimension           int                         `json:"dimension"`
	KeepAlive           string                      `json:"keep_alive"`
	EmbeddingParameters *EmbeddingParametersRequest `json:"embedding_parameters"`
	ExtraConfig         map[string]string           `json:"extra_config"`
	CustomHeaders       map[string]string           `json:"custom_headers"`
	SupportsVision      bool                        `json:"supports_vision"`
	MaxConcurrency      int                         `json:"max_concurrency"`
}

// EmbeddingParametersRequest Embedding 参数请求
type EmbeddingParametersRequest struct {
	Dimension                 int  `json:"dimension"`
	TruncatePromptTokens      int  `json:"truncate_prompt_tokens"`
	SupportsDimensionOverride bool `json:"supports_dimension_override"`
}

// UpsertModelRequest 创建/更新模型请求
type UpsertModelRequest struct {
	Name        string                 `json:"name"`
	DisplayName string                 `json:"display_name"`
	Type        string                 `json:"type"`
	Source      string                 `json:"source"`
	Description string                 `json:"description"`
	Parameters  ModelParametersRequest `json:"parameters"`
	IsDefault   bool                   `json:"is_default"`
	IsBuiltin   bool                   `json:"is_builtin"`
}

// PutModelCredentialsRequest 更新模型凭据
type PutModelCredentialsRequest struct {
	APIKey    *string `json:"api_key"`
	AppSecret *string `json:"app_secret"`
}

// SaveDocMindCloudCredentialsRequest 保存 DocMindCloud 凭据
type SaveDocMindCloudCredentialsRequest struct {
	AppID     string `json:"app_id" binding:"required"`
	AppSecret string `json:"app_secret" binding:"required"`
}

// ModelTestRequest 模型连通性测试请求
type ModelTestRequest struct {
	Source                    string            `json:"source"`
	ModelName                 string            `json:"modelName"`
	BaseURL                   string            `json:"baseUrl"`
	APIKey                    string            `json:"apiKey"`
	Provider                  string            `json:"provider"`
	ModelID                   string            `json:"modelId"`
	Dimension                 int               `json:"dimension"`
	SupportsDimensionOverride bool              `json:"supportsDimensionOverride"`
	CustomHeaders             map[string]string `json:"customHeaders"`
	ExtraConfig               map[string]string `json:"extraConfig"`
	InterfaceType             string            `json:"interfaceType"`
	AppSecret                 string            `json:"appSecret"`
}
