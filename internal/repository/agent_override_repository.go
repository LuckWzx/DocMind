package repository

import (
	"errors"

	"docmind/internal/model/entity"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type agentOverrideRepository struct {
	db *gorm.DB
}

// NewAgentOverrideRepository 创建内置智能体用户覆盖仓储
func NewAgentOverrideRepository(db *gorm.DB) AgentOverrideRepository {
	return &agentOverrideRepository{db: db}
}

func (r *agentOverrideRepository) Upsert(userID uint, agentID, name, description, avatar string, config *entity.AgentConfig) error {
	override := &entity.AgentOverride{
		UserID:      userID,
		AgentID:     agentID,
		Name:        name,
		Description: description,
		Avatar:      avatar,
	}
	if config != nil {
		override.Config = *config
	}
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "agent_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"name", "description", "avatar", "config", "updated_at"}),
	}).Create(override).Error
}

func (r *agentOverrideRepository) Delete(userID uint, agentID string) error {
	return r.db.Where("user_id = ? AND agent_id = ?", userID, agentID).Delete(&entity.AgentOverride{}).Error
}

func (r *agentOverrideRepository) Find(userID uint, agentID string) (*entity.AgentOverride, error) {
	var override entity.AgentOverride
	err := r.db.Where("user_id = ? AND agent_id = ?", userID, agentID).First(&override).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &override, nil
}

func (r *agentOverrideRepository) ListByUser(userID uint) ([]*entity.AgentOverride, error) {
	var overrides []*entity.AgentOverride
	err := r.db.Where("user_id = ?", userID).Find(&overrides).Error
	return overrides, err
}
