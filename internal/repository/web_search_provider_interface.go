package repository

import "docmind/internal/model/entity"

// WebSearchProviderRepository 网页搜索提供方仓储接口（按用户隔离）
type WebSearchProviderRepository interface {
	// FindByUserAndID 根据用户与 ID 查询（越权返回 nil）
	FindByUserAndID(userID, id uint) (*entity.WebSearchProvider, error)
	Create(p *entity.WebSearchProvider) error
	Update(p *entity.WebSearchProvider) error
	// Delete 删除（调用方需先完成归属校验）
	Delete(id uint) error
	// ListByUser 查询用户全部提供方，按创建时间倒序
	ListByUser(userID uint) ([]*entity.WebSearchProvider, error)
	// ListEnabledByUser 查询用户已启用提供方（Agent 工具构建使用）
	ListEnabledByUser(userID uint) ([]*entity.WebSearchProvider, error)
	// ClearDefault 清除用户全部默认标记（设置新默认前调用）
	ClearDefault(userID uint) error
}
