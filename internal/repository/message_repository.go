package repository

import (
	"errors"
	"slices"
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
	// 取截止 before_time 的最近 limit 条：先时间倒序 LIMIT，再反转成正序返回
	// （最早的在前）。调用方语义为"最近 N 条"，且与前端分页契约一致——
	// 打开会话显示最近消息，翻页时以 page[0].created_at 为游标向前追溯。
	err := query.Order("created_at DESC").Limit(limit).Find(&messages).Error
	slices.Reverse(messages)
	return messages, err
}

func (r *messageRepository) ListAfterID(sessionID uint, afterID uint, limit int) ([]*entity.Message, error) {
	var messages []*entity.Message
	// 按时间正序排列（最早的在前，最新的在后）
	err := r.db.Where("session_id = ? AND id > ?", sessionID, afterID).
		Order("created_at ASC").
		Limit(limit).
		Find(&messages).Error
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

func (r *messageRepository) CountUserTurnsBySession(sessionID uint) (int64, error) {
	var count int64
	err := r.db.Model(&entity.Message{}).Where("session_id = ? AND role = 'user'", sessionID).Count(&count).Error
	return count, err
}
