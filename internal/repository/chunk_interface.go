package repository

import "docmind/internal/model/entity"

// ChunkRepository 分块仓储接口
type ChunkRepository interface {
	ListByKnowledgeBase(knowledgeBaseID uint, chunkIDs []uint) ([]*entity.Chunk, error)
	ListByIDs(chunkIDs []uint) ([]*entity.Chunk, error)
	UpdateStatusByIDs(chunkIDs []uint, status int) error
}
