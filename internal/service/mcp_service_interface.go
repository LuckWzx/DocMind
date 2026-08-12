package service

import (
	"docmind/internal/model/dto/request"
	dto "docmind/internal/model/dto/response"
	"docmind/internal/model/entity"
)

// MCPServiceService MCP 服务业务接口
type MCPServiceService interface {
	// List 获取用户（含内置）全部 MCP 服务（数量少，不分页，对齐前端契约数组返回）
	List(userID uint) ([]*dto.MCPServiceResponse, error)
	GetByID(userID, id uint) (*dto.MCPServiceResponse, error)
	Create(userID uint, req *request.CreateMCPServiceRequest) (*dto.MCPServiceResponse, error)
	Update(userID, id uint, req *request.UpdateMCPServiceRequest) (*dto.MCPServiceResponse, error)
	Delete(userID, id uint) error
	Test(userID, id uint) (*dto.MCPTestResultResponse, error)
	ListTools(userID, id uint) ([]dto.MCPToolResponse, error)
	ListResources(userID, id uint) ([]dto.MCPResourceResponse, error)
	// UpdateCredentials 更新服务凭据（密钥子资源，不返回密钥本身）
	UpdateCredentials(userID, id uint, fields map[string]string) (*dto.McpCredentialsResponse, error)
	// DeleteCredentialField 清除指定凭据字段
	DeleteCredentialField(userID, id uint, field string) (*dto.McpCredentialsResponse, error)
	// ListEnabledByUser 查询用户（含内置）已启用的 MCP 服务，Agent 工具集构建使用
	ListEnabledByUser(userID uint) ([]*entity.MCPService, error)
	// GetEntityByUser 按用户查询 MCP 服务实体（含内置），Agent 工具集构建使用
	GetEntityByUser(userID, id uint) (*entity.MCPService, error)
	// GetToolApprovals 查询用户对指定服务的工具审批设置
	GetToolApprovals(userID, serviceID uint) ([]*dto.MCPToolApprovalResponse, error)
	// SetToolApproval 设置（或清除）指定工具的审批要求
	SetToolApproval(userID, serviceID uint, toolName string, requireApproval bool) error
}
