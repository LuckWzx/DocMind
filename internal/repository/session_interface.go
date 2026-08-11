package repository

import (
	"docmind/internal/model/entity"
	"time"
)

// SessionRepository 会话仓储接口
type SessionRepository interface {
	Create(session *entity.Session) error
	Update(session *entity.Session) error
	Delete(id uint) error
	FindByID(id uint) (*entity.Session, error)
	ListByUser(userID uint, source string, page, pageSize int) ([]*entity.Session, int64, error)
	UpdatePin(id uint, pinned bool) error
	// UpdateModeState 更新会话的对话模式状态（AgentID/AgentEnabled/LastRequestState 快照），
	// 部分更新不触碰其他字段；agentID 为空时保留原值（请求未携带智能体时不清空绑定）
	UpdateModeState(id uint, agentID string, enabled bool, state *entity.SessionLastRequestState) error
	IncrementMessageCount(id uint) error
	UpdateLastMessage(id uint, preview string) error
	CountBySession(sessionID uint) (int64, error)
}

// MessageRepository 消息仓储接口
type MessageRepository interface {
	Create(message *entity.Message) error
	BatchCreate(messages []*entity.Message) error
	FindByID(id uint) (*entity.Message, error)
	ListBySession(sessionID uint, limit int, beforeTime *time.Time) ([]*entity.Message, error)
	// ListAfterID 加载 ID 大于 afterID 的消息（增量加载：短期记忆压缩边界之后的消息）
	ListAfterID(sessionID uint, afterID uint, limit int) ([]*entity.Message, error)
	DeleteBySession(sessionID uint) error
	CountBySession(sessionID uint) (int64, error)
}
