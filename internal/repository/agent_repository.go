package repository

import (
	"errors"
	"fmt"

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
	// 首先尝试通过 id_str 字段查找
	err := r.db.Where("id_str = ?", idStr).First(&agent).Error
	fmt.Printf("[AgentRepo] FindByIDStr: idStr=%s, err=%v\n", idStr, err)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			fmt.Printf("[AgentRepo] FindByIDStr: id_str 未找到，尝试用数字 ID 查找\n")
			// 如果 id_str 没有找到，尝试通过数字 ID 查找（前端可能传递的是数字 ID 的字符串形式）
			var agentByID entity.Agent
			errByID := r.db.Where("id = ?", idStr).First(&agentByID).Error
			fmt.Printf("[AgentRepo] FindByIDStr: id=%s, errByID=%v\n", idStr, errByID)
			if errByID != nil {
				if errors.Is(errByID, gorm.ErrRecordNotFound) {
					fmt.Printf("[AgentRepo] FindByIDStr: 数字 ID 也未找到\n")
					return nil, nil
				}
				return nil, errByID
			}
			return &agentByID, nil
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
