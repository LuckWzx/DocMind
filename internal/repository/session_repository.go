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

// UpdateModeState 部分更新会话的对话模式状态：AgentID（非空时）/ AgentEnabled / LastRequestState 快照。
// 使用 map 更新避免 GORM Save 全字段覆盖；LastRequestState 为 nil 时更新为 NULL（清空快照）。
func (r *sessionRepository) UpdateModeState(id uint, agentID string, enabled bool, state *entity.SessionLastRequestState) error {
	fields := map[string]interface{}{
		"agent_enabled":      enabled,
		"last_request_state": state,
	}
	if agentID != "" {
		fields["agent_id"] = agentID
	}
	return r.db.Model(&entity.Session{}).Where("id = ?", id).Updates(fields).Error
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

// TouchAfterMessage 消息落库后同步更新会话：message_count+1 与 last_message 合并为单条 UPDATE，
// 替代原先 IncrementMessageCount + UpdateLastMessage 两次数据库往返。
func (r *sessionRepository) TouchAfterMessage(id uint, preview string) error {
	runes := []rune(preview)
	if len(runes) > 200 {
		preview = string(runes[:200])
	}
	return r.db.Model(&entity.Session{}).Where("id = ?", id).Updates(map[string]interface{}{
		"message_count": gorm.Expr("message_count + 1"),
		"last_message":  preview,
	}).Error
}

func (r *sessionRepository) CountBySession(sessionID uint) (int64, error) {
	var count int64
	err := r.db.Model(&entity.Message{}).Where("session_id = ?", sessionID).Count(&count).Error
	return count, err
}

// ListOwnedIDs 返回属于该用户的会话 ID；ids 非空时仅返回其中归属于该用户的（用于越权校验）。
func (r *sessionRepository) ListOwnedIDs(userID uint, ids []uint) ([]uint, error) {
	q := r.db.Model(&entity.Session{}).Where("user_id = ?", userID)
	if len(ids) > 0 {
		q = q.Where("id IN ?", ids)
	}
	var owned []uint
	err := q.Pluck("id", &owned).Error
	return owned, err
}

// DeleteByIDs 批量删除会话
func (r *sessionRepository) DeleteByIDs(ids []uint) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.Where("id IN ?", ids).Delete(&entity.Session{}).Error
}
