package response

import "time"

// CredentialFieldMetadata 凭据字段是否已配置（响应中不返回密钥本身）
type CredentialFieldMetadata struct {
	Configured bool `json:"configured"`
}

// MCPServiceAuthConfigResponse 认证配置响应（密钥字段脱敏，仅标记是否已配置）
type MCPServiceAuthConfigResponse struct {
	AuthType      string            `json:"auth_type,omitempty"`      // '' / api_key / bearer
	APIKeyHeader  string            `json:"api_key_header,omitempty"` // api_key 的请求头名
	CustomHeaders map[string]string `json:"custom_headers,omitempty"`
}

// MCPServiceAdvancedConfigResponse 高级配置响应
type MCPServiceAdvancedConfigResponse struct {
	Timeout    int `json:"timeout,omitempty"`
	RetryCount int `json:"retry_count,omitempty"`
	RetryDelay int `json:"retry_delay,omitempty"`
}

// MCPServiceStdioConfigResponse stdio 传输配置响应
type MCPServiceStdioConfigResponse struct {
	Command string   `json:"command,omitempty"`
	Args    []string `json:"args,omitempty"`
}

// MCPServiceResponse MCP 服务响应
type MCPServiceResponse struct {
	ID             uint                               `json:"id"`
	Name           string                             `json:"name"`
	Description    string                             `json:"description,omitempty"`
	Enabled        bool                               `json:"enabled"`
	TransportType  string                             `json:"transport_type"`
	URL            string                             `json:"url,omitempty"`
	Headers        map[string]string                  `json:"headers,omitempty"`
	AuthConfig     *MCPServiceAuthConfigResponse      `json:"auth_config,omitempty"`
	AdvancedConfig *MCPServiceAdvancedConfigResponse  `json:"advanced_config,omitempty"`
	StdioConfig    *MCPServiceStdioConfigResponse     `json:"stdio_config,omitempty"`
	EnvVars        map[string]string                  `json:"env_vars,omitempty"`
	IsBuiltin      bool                               `json:"is_builtin,omitempty"`
	Credentials    map[string]CredentialFieldMetadata `json:"credentials,omitempty"`
	CreatedAt      time.Time                          `json:"created_at"`
	UpdatedAt      time.Time                          `json:"updated_at"`
}

// MCPToolResponse MCP 工具元数据
type MCPToolResponse struct {
	Name            string      `json:"name"`
	Description     string      `json:"description,omitempty"`
	InputSchema     interface{} `json:"input_schema,omitempty"`
	RequireApproval bool        `json:"require_approval,omitempty"` // v1 恒为 false，字段保留供前端契约对齐
}

// MCPResourceResponse MCP 资源元数据
type MCPResourceResponse struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

// MCPTestResultResponse MCP 服务连通性测试结果
type MCPTestResultResponse struct {
	Success   bool                  `json:"success"`
	Message   string                `json:"message,omitempty"`
	Tools     []MCPToolResponse     `json:"tools,omitempty"`
	Resources []MCPResourceResponse `json:"resources,omitempty"`
}

// McpCredentialsResponse 凭据子资源响应（密钥本身不返回，仅标记各字段是否已配置）
type McpCredentialsResponse struct {
	Fields map[string]CredentialFieldMetadata `json:"fields"`
}
