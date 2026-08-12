package repository

import "docmind/internal/model/entity"

// MCPToolApprovalRepository MCP 工具审批设置仓储接口
// 审批偏好按用户存储：用户可对可见服务（含全局）的每个工具设置是否需要人工审批
type MCPToolApprovalRepository interface {
	// ListByUserAndService 查询用户对指定服务的全部审批设置
	ListByUserAndService(userID, serviceID uint) ([]*entity.MCPToolApproval, error)
	// ListByUser 查询用户全部审批设置（Agent 工具集构建时一次拉全量）
	ListByUser(userID uint) ([]*entity.MCPToolApproval, error)
	// Upsert 设置（或清除）某工具的审批要求，按 (UserID, ServiceID, ToolName) 唯一
	Upsert(userID, serviceID uint, toolName string, requireApproval bool) error
}
