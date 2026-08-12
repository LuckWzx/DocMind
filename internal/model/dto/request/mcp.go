package request

// MCPServiceListRequest MCP 服务列表请求
type MCPServiceListRequest struct {
	Page int `form:"page"` // 页码，从1开始，默认1
	Size int `form:"size"` // 每页大小，默认20
}

// MCPServiceAuthConfigRequest 认证配置请求
type MCPServiceAuthConfigRequest struct {
	AuthType      string            `json:"auth_type" binding:"omitempty,oneof=api_key bearer oauth"` // '' / api_key / bearer / oauth（oauth 流程 v1 未实现，仅存储）
	APIKey        string            `json:"api_key"`                                                  // 敏感，仅写入
	APIKeyHeader  string            `json:"api_key_header"`                                           // api_key 的请求头名，默认 X-API-Key
	Token         string            `json:"token"`                                                    // 敏感，仅写入
	CustomHeaders map[string]string `json:"custom_headers"`
}

// MCPServiceAdvancedConfigRequest 高级配置请求
type MCPServiceAdvancedConfigRequest struct {
	Timeout    int `json:"timeout"`     // 单次请求超时（秒）
	RetryCount int `json:"retry_count"` // 重试次数
	RetryDelay int `json:"retry_delay"` // 重试间隔（秒）
}

// MCPServiceStdioConfigRequest stdio 传输配置请求
type MCPServiceStdioConfigRequest struct {
	Command string   `json:"command" binding:"required"` // 启动命令（如 npx / uvx）
	Args    []string `json:"args"`
}

// CreateMCPServiceRequest 创建 MCP 服务请求
type CreateMCPServiceRequest struct {
	Name           string                           `json:"name" binding:"required,min=1,max=128"`
	Description    string                           `json:"description"`
	Enabled        *bool                            `json:"enabled"`
	TransportType  string                           `json:"transport_type" binding:"required,oneof=sse stdio http-streamable"`
	URL            string                           `json:"url"` // SSE / http-streamable 传输必填
	Headers        map[string]string                `json:"headers"`
	AuthConfig     *MCPServiceAuthConfigRequest     `json:"auth_config"`
	AdvancedConfig *MCPServiceAdvancedConfigRequest `json:"advanced_config"`
	StdioConfig    *MCPServiceStdioConfigRequest    `json:"stdio_config"`
	EnvVars        map[string]string                `json:"env_vars"` // stdio 传输的子进程环境变量
}

// UpdateMCPServiceRequest 更新 MCP 服务请求（字段为空表示不修改）
type UpdateMCPServiceRequest struct {
	Name           string                           `json:"name" binding:"omitempty,min=1,max=128"`
	Description    string                           `json:"description"`
	Enabled        *bool                            `json:"enabled"`
	TransportType  string                           `json:"transport_type" binding:"omitempty,oneof=sse stdio http-streamable"`
	URL            string                           `json:"url"`
	Headers        map[string]string                `json:"headers"`
	AuthConfig     *MCPServiceAuthConfigRequest     `json:"auth_config"`
	AdvancedConfig *MCPServiceAdvancedConfigRequest `json:"advanced_config"`
	StdioConfig    *MCPServiceStdioConfigRequest    `json:"stdio_config"`
	EnvVars        map[string]string                `json:"env_vars"`
}

// UpdateMCPCredentialsRequest 更新凭据子资源请求（指针区分未提交与清空）
type UpdateMCPCredentialsRequest struct {
	APIKey *string `json:"api_key"` // 非空则覆盖写入
	Token  *string `json:"token"`   // 非空则覆盖写入
}

// SetMCPToolApprovalRequest 设置工具审批要求请求
// require_approval=true 时 Agent 调用该工具前会被拦截，false 表示放行
type SetMCPToolApprovalRequest struct {
	RequireApproval bool `json:"require_approval"`
}
