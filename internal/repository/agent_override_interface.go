package repository

import "docmind/internal/model/entity"

// AgentOverrideRepository 内置智能体用户覆盖仓储
type AgentOverrideRepository interface {
	// Upsert 创建或更新覆盖（不存在则创建，存在则更新）
	Upsert(userID uint, agentID, name, description, avatar string, config *entity.AgentConfig) error
	// Delete 删除覆盖（恢复默认）
	Delete(userID uint, agentID string) error
	// Find 查询单个覆盖
	Find(userID uint, agentID string) (*entity.AgentOverride, error)
	// ListByUser 查询用户全部覆盖
	ListByUser(userID uint) ([]*entity.AgentOverride, error)
}
