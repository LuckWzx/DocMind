package repository

import (
	"errors"
	"reflect"

	"docmind/internal/model/entity"
	"docmind/pkg/logger"

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
	// 优先按 id_str 字段查找（内置智能体为 builtin-xxx 标识）
	err := r.db.Where("id_str = ?", idStr).First(&agent).Error
	if err == nil {
		return &agent, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	// id_str 未命中时兑底按数字主键 ID 查找（前端可能传数字 ID 字符串，兼容旧数据）
	var agentByID entity.Agent
	if errByID := r.db.Where("id = ?", idStr).First(&agentByID).Error; errByID == nil {
		return &agentByID, nil
	} else if !errors.Is(errByID, gorm.ErrRecordNotFound) {
		return nil, errByID
	}
	// 两种标识均未找到：debug 级日志（常规兜底路径，避免每次会话解析都刷 warn）
	logger.Debugf("[AgentRepo] FindByIDStr: id_str=%s 未找到", idStr)
	return nil, nil
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

// EnsureBuiltin 确保内置智能体存在；模板（Config 等）变更时同步更新内置行
// 用户个性化保存在独立的 override 表，不受此同步影响
func (r *agentRepository) EnsureBuiltin(agent *entity.Agent) error {
	existing, err := r.FindByIDStr(agent.IDStr)
	if err != nil {
		return err
	}
	if existing == nil {
		return r.db.Create(agent).Error
	}
	// 模板变更同步：仅更新内置行自身字段，不触碰 override
	if existing.Name != agent.Name || existing.Description != agent.Description ||
		existing.Avatar != agent.Avatar || !reflect.DeepEqual(existing.Config, agent.Config) {
		existing.Name = agent.Name
		existing.Description = agent.Description
		existing.Avatar = agent.Avatar
		existing.Config = agent.Config
		return r.db.Save(existing).Error
	}
	return nil
}
