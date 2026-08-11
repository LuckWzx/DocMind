package repository

import (
	"errors"

	"docmind/internal/model/entity"

	"gorm.io/gorm"
)

// mcpServiceRepository MCP 服务仓储实现
type mcpServiceRepository struct {
	db *gorm.DB
}

// NewMCPServiceRepository 创建 MCP 服务仓储
func NewMCPServiceRepository(db *gorm.DB) MCPServiceRepository {
	return &mcpServiceRepository{db: db}
}

// FindByID 根据 ID 查询 MCP 服务
func (r *mcpServiceRepository) FindByID(id uint) (*entity.MCPService, error) {
	var svc entity.MCPService
	err := r.db.First(&svc, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &svc, nil
}

// FindByUserAndID 根据用户与 ID 查询 MCP 服务（user_id=0 的内置服务对任何用户可见）
func (r *mcpServiceRepository) FindByUserAndID(userID, id uint) (*entity.MCPService, error) {
	var svc entity.MCPService
	err := r.db.Where("(user_id = ? OR user_id = 0) AND id = ?", userID, id).First(&svc).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &svc, nil
}

// Create 创建 MCP 服务
func (r *mcpServiceRepository) Create(svc *entity.MCPService) error {
	return r.db.Create(svc).Error
}

// Update 更新 MCP 服务
func (r *mcpServiceRepository) Update(svc *entity.MCPService) error {
	return r.db.Save(svc).Error
}

// Delete 删除 MCP 服务
func (r *mcpServiceRepository) Delete(id uint) error {
	return r.db.Delete(&entity.MCPService{}, id).Error
}

// ListByUser 查询用户 MCP 服务（含 user_id=0 内置），按创建时间倒序
func (r *mcpServiceRepository) ListByUser(userID uint) ([]*entity.MCPService, error) {
	var svcs []*entity.MCPService
	err := r.db.Where("user_id = ? OR user_id = 0", userID).
		Order("user_id ASC, created_at DESC").
		Find(&svcs).Error
	if err != nil {
		return nil, err
	}
	return svcs, nil
}

// ListEnabledByUser 查询用户（含内置）已启用的 MCP 服务，Agent 构建工具集时使用
func (r *mcpServiceRepository) ListEnabledByUser(userID uint) ([]*entity.MCPService, error) {
	var svcs []*entity.MCPService
	err := r.db.Where("(user_id = ? OR user_id = 0) AND enabled = true", userID).
		Order("user_id ASC, id ASC").
		Find(&svcs).Error
	if err != nil {
		return nil, err
	}
	return svcs, nil
}
