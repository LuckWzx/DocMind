package entity

// MCPToolApproval MCP 工具人工审批设置（用户级偏好）
// 语义：require_approval=true 的工具在 Agent 调用前会被拦截，返回提示
// 按 (UserID, ServiceID, ToolName) 唯一；用户对全局服务也可设置自己的审批偏好，
// 该表不属于服务配置本身，不受服务"全局只读"保护约束
type MCPToolApproval struct {
	BaseEntity
	UserID          uint   `gorm:"index:idx_mcp_approval,unique,priority:1;not null;comment:用户ID" json:"user_id"`
	ServiceID       uint   `gorm:"index:idx_mcp_approval,unique,priority:2;not null;comment:MCP服务ID" json:"service_id"`
	ToolName        string `gorm:"type:varchar(255);index:idx_mcp_approval,unique,priority:3;not null;comment:工具名(原始名，不含前缀)" json:"tool_name"`
	RequireApproval bool   `gorm:"default:false;comment:调用前是否需要人工审批" json:"require_approval"`
}

// TableName 指定表名
func (MCPToolApproval) TableName() string {
	return "mcp_tool_approvals"
}
