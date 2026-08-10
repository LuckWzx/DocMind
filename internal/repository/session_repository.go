package repository

import (
	"errors"

	"docmind/internal/model/entity"

	"gorm.io/gorm"
)

type sessionRepository struct {
	db *gorm.DB
}

// NewSessionRepository 创建会话仓储
func NewSessionRepository(db *gorm.DB) SessionRepository {
	return &sessionRepository{db: db}
}

func (r *sessionRepository) Create(session *entity.Session) error {
	return r.db.Create(session).Error
}

func (r *sessionRepository) Update(session *entity.Session) error {
	return r.db.Save(session).Error
}

func (r *sessionRepository) Delete(id uint) error {
	return r.db.Delete(&entity.Session{}, id).Error
}

func (r *sessionRepository) FindByID(id uint) (*entity.Session, error) {
	var session entity.Session
	err := r.db.First(&session, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &session, nil
}

func (r *sessionRepository) ListByUser(userID uint, source string, page, pageSize int) ([]*entity.Session, int64, error) {
	var sessions []*entity.Session
	var total int64

	query := r.db.Model(&entity.Session{}).Where("user_id = ?", userID)
	if source != "" {
		query = query.Where("source = ?", source)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Order("pinned DESC").Order("updated_at DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Find(&sessions).Error
	return sessions, total, err
}

func (r *sessionRepository) UpdatePin(id uint, pinned bool) error {
	return r.db.Model(&entity.Session{}).Where("id = ?", id).
		Update("pinned", pinned).Error
}

func (r *sessionRepository) IncrementMessageCount(id uint) error {
	return r.db.Model(&entity.Session{}).Where("id = ?", id).
		UpdateColumn("message_count", gorm.Expr("message_count + 1")).Error
}

func (r *sessionRepository) UpdateLastMessage(id uint, preview string) error {
	runes := []rune(preview)
	if len(runes) > 200 {
		preview = string(runes[:200])
	}
	return r.db.Model(&entity.Session{}).Where("id = ?", id).
		Update("last_message", preview).Error
}

func (r *sessionRepository) CountBySession(sessionID uint) (int64, error) {
	var count int64
	err := r.db.Model(&entity.Message{}).Where("session_id = ?", sessionID).Count(&count).Error
	return count, err
}
