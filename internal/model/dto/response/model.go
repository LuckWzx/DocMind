package response

// CredentialFieldState 凭据字段状态
type CredentialFieldState struct {
	Configured bool `json:"configured"`
}

// ModelCredentialsResponse 模型凭据状态响应
type ModelCredentialsResponse struct {
	Fields map[string]CredentialFieldState `json:"fields"`
}

// EmbeddingParametersResponse Embedding 参数响应
type EmbeddingParametersResponse struct {
	Dimension                 int  `json:"dimension,omitempty"`
	TruncatePromptTokens      int  `json:"truncate_prompt_tokens,omitempty"`
	SupportsDimensionOverride bool `json:"supports_dimension_override,omitempty"`
}

// ModelParametersResponse 模型参数响应（已脱敏）
type ModelParametersResponse struct {
	BaseURL             string                       `json:"base_url,omitempty"`
	APIVersion          string                       `json:"api_version,omitempty"`
	ModelName           string                       `json:"model_name,omitempty"`
	Provider            string                       `json:"provider,omitempty"`
	InterfaceType       string                       `json:"interface_type,omitempty"`
	ParameterSize       string                       `json:"parameter_size,omitempty"`
	Temperature         float64                      `json:"temperature,omitempty"`
	MaxTokens           int                          `json:"max_tokens,omitempty"`
	ContextWindow       int                          `json:"context_window,omitempty"`
	Dimension           int                          `json:"dimension,omitempty"`
	KeepAlive           string                       `json:"keep_alive,omitempty"`
	EmbeddingParameters *EmbeddingParametersResponse `json:"embedding_parameters,omitempty"`
	ExtraConfig         map[string]string            `json:"extra_config,omitempty"`
	CustomHeaders       map[string]string            `json:"custom_headers,omitempty"`
	SupportsVision      bool                         `json:"supports_vision,omitempty"`
	MaxConcurrency      int                          `json:"max_concurrency,omitempty"`
	AppID               string                       `json:"app_id,omitempty"`
}

// ModelResponse 模型响应
type ModelResponse struct {
	ID          string                          `json:"id"`
	Name        string                          `json:"name"`
	DisplayName string                          `json:"display_name,omitempty"`
	Type        string                          `json:"type"`
	Source      string                          `json:"source"`
	Description string                          `json:"description,omitempty"`
	Status      string                          `json:"status,omitempty"`
	IsDefault   bool                            `json:"is_default"`
	IsBuiltin   bool                            `json:"is_builtin"`
	Parameters  ModelParametersResponse         `json:"parameters"`
	Credentials map[string]CredentialFieldState `json:"credentials,omitempty"`
	CreatedAt   string                          `json:"created_at"`
	UpdatedAt   string                          `json:"updated_at"`
}

// ModelProviderOptionResponse 模型厂商选项
type ModelProviderOptionResponse struct {
	Value       string            `json:"value"`
	Label       string            `json:"label"`
	Description string            `json:"description"`
	DefaultURLs map[string]string `json:"defaultUrls"`
	ModelTypes  []string          `json:"modelTypes"`
}

// OllamaModelInfoResponse Ollama 模型详情
type OllamaModelInfoResponse struct {
	Name       string `json:"name"`
	Size       int64  `json:"size"`
	Digest     string `json:"digest"`
	ModifiedAt string `json:"modified_at"`
}

// OllamaStatusResponse Ollama 服务状态
type OllamaStatusResponse struct {
	Available bool   `json:"available"`
	Version   string `json:"version,omitempty"`
	Error     string `json:"error,omitempty"`
	BaseURL   string `json:"baseUrl,omitempty"`
}

// DownloadTaskResponse Ollama 下载任务
type DownloadTaskResponse struct {
	ID        string  `json:"id"`
	ModelName string  `json:"modelName"`
	Status    string  `json:"status"`
	Progress  float64 `json:"progress"`
	Message   string  `json:"message"`
	StartTime string  `json:"startTime"`
	EndTime   string  `json:"endTime,omitempty"`
}

// DocMindCloudStatusResponse DocMindCloud 凭据状态
type DocMindCloudStatusResponse struct {
	HasModels   bool   `json:"has_models"`
	NeedsReinit bool   `json:"needs_reinit"`
	Reason      string `json:"reason,omitempty"`
}

// ModelDebugResult 调试结果
type ModelDebugResult struct {
	OK           bool                   `json:"ok"`
	ElapsedMS    int64                  `json:"elapsed_ms"`
	Request      map[string]interface{} `json:"request"`
	RawResponse  interface{}            `json:"raw_response"`
	Observations map[string]interface{} `json:"observations"`
	Error        string                 `json:"error,omitempty"`
}

// ModelContextWindowMissingResponse 上下文大小缺失记录响应（待补足映射表的模型清单）
type ModelContextWindowMissingResponse struct {
	ID        uint   `json:"id"`
	ModelID   uint   `json:"model_id"`
	ModelName string `json:"model_name"`
	Provider  string `json:"provider,omitempty"`
	BaseURL   string `json:"base_url,omitempty"`
	Source    string `json:"source,omitempty"`
	Type      string `json:"type,omitempty"`
	Reason    string `json:"reason,omitempty"`
	CreatedAt string `json:"created_at"`
}
