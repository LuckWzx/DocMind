package repository

import "docmind/internal/model/entity"

// SummaryRepository 会话摘要仓储（短期记忆增量压缩的持久化状态）
type SummaryRepository interface {
	// GetBySession 读取会话摘要，不存在返回 nil, nil
	GetBySession(sessionID uint) (*entity.SessionSummary, error)
	// Upsert 写入或更新会话摘要（每个会话至多一行）
	Upsert(summary *entity.SessionSummary) error
	// DeleteBySession 删除会话摘要（删除/清空会话时联动清理）
	DeleteBySession(sessionID uint) error
}
