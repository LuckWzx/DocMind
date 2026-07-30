package repository

import (
	"errors"

	"docmind/internal/model/entity"

	"gorm.io/gorm"
)

type agentRepository struct {
	db *gorm.DB
}

// NewAgentRepository 创建智能体仓储
func NewAgentRepository(db *gorm.DB) AgentRepository {
	return &agentRepository{db: db}
}

func (r *agentRepository) Create(agent *entity.Agent) error {
	return r.db.Create(agent).Error
}

func (r *agentRepository) Update(agent *entity.Agent) error {
	return r.db.Save(agent).Error
}

func (r *agentRepository) Delete(id uint) error {
	return r.db.Delete(&entity.Agent{}, id).Error
}

func (r *agentRepository) FindByID(id uint) (*entity.Agent, error) {
	var agent entity.Agent
	err := r.db.First(&agent, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &agent, nil
}

func (r *agentRepository) FindByIDStr(idStr string) (*entity.Agent, error) {
	var agent entity.Agent
	err := r.db.Where("id_str = ?", idStr).First(&agent).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &agent, nil
}

func (r *agentRepository) ListByUser(userID uint) ([]*entity.Agent, error) {
	var agents []*entity.Agent
	err := r.db.Where("user_id = ? OR user_id = 0", userID).
		Order("is_builtin DESC").Order("created_at DESC").
		Find(&agents).Error
	return agents, err
}

func (r *agentRepository) ListAll() ([]*entity.Agent, error) {
	var agents []*entity.Agent
	err := r.db.Order("is_builtin DESC").Order("created_at DESC").Find(&agents).Error
	return agents, err
}

// EnsureBuiltin 确保内置智能体存在（不存在则创建）
func (r *agentRepository) EnsureBuiltin(agent *entity.Agent) error {
	existing, err := r.FindByIDStr(agent.IDStr)
	if err != nil {
		return err
	}
	if existing != nil {
		return nil // 已存在
	}
	return r.db.Create(agent).Error
}
