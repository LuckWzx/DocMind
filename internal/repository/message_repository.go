package repository

import (
	"errors"
	"time"

	"docmind/internal/model/entity"

	"gorm.io/gorm"
)

type messageRepository struct {
	db *gorm.DB
}

// NewMessageRepository 创建消息仓储
func NewMessageRepository(db *gorm.DB) MessageRepository {
	return &messageRepository{db: db}
}

func (r *messageRepository) Create(message *entity.Message) error {
	return r.db.Create(message).Error
}

func (r *messageRepository) BatchCreate(messages []*entity.Message) error {
	if len(messages) == 0 {
		return nil
	}
	return r.db.CreateInBatches(messages, 100).Error
}

func (r *messageRepository) FindByID(id uint) (*entity.Message, error) {
	var msg entity.Message
	err := r.db.First(&msg, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &msg, nil
}

func (r *messageRepository) ListBySession(sessionID uint, limit int, beforeTime *time.Time) ([]*entity.Message, error) {
	var messages []*entity.Message
	query := r.db.Where("session_id = ?", sessionID)
	if beforeTime != nil {
		query = query.Where("created_at < ?", *beforeTime)
	}
	// 按时间正序排列（最早的在前，最新的在后）
	err := query.Order("created_at ASC").Limit(limit).Find(&messages).Error
	return messages, err
}

func (r *messageRepository) DeleteBySession(sessionID uint) error {
	return r.db.Where("session_id = ?", sessionID).Delete(&entity.Message{}).Error
}

func (r *messageRepository) CountBySession(sessionID uint) (int64, error) {
	var count int64
	err := r.db.Model(&entity.Message{}).Where("session_id = ?", sessionID).Count(&count).Error
	return count, err
}
