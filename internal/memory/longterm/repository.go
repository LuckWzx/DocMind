package longterm

import "context"

// MemoryRepository 长期记忆图仓储接口。
// Neo4j 不可用时由 service 层通过 IsAvailable 降级跳过，不阻断主流程。
type MemoryRepository interface {
	// IsAvailable 图数据库是否可用（连接建立成功）
	IsAvailable() bool
	// SaveEpisode 保存记忆片段：Episode 节点 + MENTIONS 实体 + RELATED_TO 关系（单事务）
	SaveEpisode(ctx context.Context, episode *Episode, entities []*Entity, relations []*Relationship) error
	// FindRelatedEpisodes 按关键词匹配实体，返回关联的 Episode（按时间倒序）
	FindRelatedEpisodes(ctx context.Context, userID uint, keywords []string, limit int) ([]*Episode, error)
	// Close 关闭连接
	Close(ctx context.Context) error
}
