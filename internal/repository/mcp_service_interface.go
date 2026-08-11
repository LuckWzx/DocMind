package repository

import "docmind/internal/model/entity"

// MCPServiceRepository MCP 服务仓储接口
type MCPServiceRepository interface {
	FindByID(id uint) (*entity.MCPService, error)
	FindByUserAndID(userID, id uint) (*entity.MCPService, error)
	Create(svc *entity.MCPService) error
	Update(svc *entity.MCPService) error
	Delete(id uint) error
	// ListByUser 查询用户（含 user_id=0 内置）的 MCP 服务，按创建时间倒序
	ListByUser(userID uint) ([]*entity.MCPService, error)
	// ListEnabledByUser 查询用户（含内置）已启用的 MCP 服务，Agent 构建工具集时使用
	ListEnabledByUser(userID uint) ([]*entity.MCPService, error)
}
