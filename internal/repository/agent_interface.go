package repository

import "docmind/internal/model/entity"

// AgentRepository 智能体仓储接口
type AgentRepository interface {
	Create(agent *entity.Agent) error
	Update(agent *entity.Agent) error
	Delete(id uint) error
	FindByID(id uint) (*entity.Agent, error)
	FindByIDStr(idStr string) (*entity.Agent, error)
	ListByUser(userID uint) ([]*entity.Agent, error)
	ListAll() ([]*entity.Agent, error)
	EnsureBuiltin(agent *entity.Agent) error
}
