package entity

// MCP 传输类型
const (
	MCPTransportSSE            = "sse"
	MCPTransportStdio          = "stdio"
	MCPTransportHTTPStreamable = "http-streamable" // 新版远程规范（2025-03-26），单 POST 端点，取代 SSE
)

// MCP 认证类型
const (
	MCPAuthTypeNone   = ""
	MCPAuthTypeAPIKey = "api_key"
	MCPAuthTypeBearer = "bearer"
	MCPAuthTypeOAuth  = "oauth" // v1 未实现授权码流程，创建/连接层明确拒绝
)

// MCPServiceAuthConfig MCP 服务认证配置
type MCPServiceAuthConfig struct {
	AuthType      string            `json:"auth_type,omitempty"`      // '' / api_key / bearer
	APIKey        string            `json:"api_key,omitempty"`        // 敏感，响应脱敏
	APIKeyHeader  string            `json:"api_key_header,omitempty"` // api_key 的请求头名，默认 X-API-Key
	Token         string            `json:"token,omitempty"`          // 敏感，响应脱敏
	CustomHeaders map[string]string `json:"custom_headers,omitempty"` // 自定义请求头
}

// MCPServiceAdvancedConfig MCP 服务高级配置
type MCPServiceAdvancedConfig struct {
	Timeout    int `json:"timeout,omitempty"`     // 单次请求超时（秒）
	RetryCount int `json:"retry_count,omitempty"` // 重试次数
	RetryDelay int `json:"retry_delay,omitempty"` // 重试间隔（秒）
}

// MCPServiceStdioConfig stdio 传输配置
type MCPServiceStdioConfig struct {
	Command string   `json:"command,omitempty"` // 启动命令（如 npx / uvx）
	Args    []string `json:"args,omitempty"`    // 命令参数
}

// MCPServiceTool MCP 工具元数据（ListTools 结果缓存）
type MCPServiceTool struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	InputSchema interface{} `json:"input_schema,omitempty"`
}

// MCPServiceResource MCP 资源元数据
type MCPServiceResource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mime_type,omitempty"`
}

// MCPService 外部 MCP Server 注册信息，Agent 按配置加载其工具
type MCPService struct {
	BaseEntity
	UserID         uint   `gorm:"index;default:0;not null;comment:所属用户ID(0=内置)" json:"user_id"`
	Name           string `gorm:"type:varchar(128);not null;comment:服务名称" json:"name"`
	Description    string `gorm:"type:text;comment:服务描述" json:"description"`
	Enabled        bool   `gorm:"default:true;comment:是否启用" json:"enabled"`
	TransportType  string `gorm:"type:varchar(32);not null;comment:传输类型 sse/stdio" json:"transport_type"`
	URL            string `gorm:"type:varchar(512);comment:服务地址(SSE传输)" json:"url,omitempty"`
	Headers        JSON   `gorm:"type:jsonb;comment:自定义请求头" json:"headers,omitempty"`
	AuthConfig     JSON   `gorm:"type:jsonb;comment:认证配置" json:"auth_config,omitempty"`
	AdvancedConfig JSON   `gorm:"type:jsonb;comment:高级配置" json:"advanced_config,omitempty"`
	StdioConfig    JSON   `gorm:"type:jsonb;comment:stdio传输配置" json:"stdio_config,omitempty"`
	EnvVars        JSON   `gorm:"type:jsonb;comment:环境变量(stdio)" json:"env_vars,omitempty"`
	ToolsCache     JSON   `gorm:"type:jsonb;comment:工具列表缓存(ListTools结果)" json:"tools_cache,omitempty"`
}

// TableName 指定表名
func (MCPService) TableName() string {
	return "mcp_services"
}
