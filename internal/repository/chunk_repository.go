package repository

import (
	"docmind/internal/model/entity"

	"gorm.io/gorm"
)

type chunkRepository struct {
	db *gorm.DB
}

// NewChunkRepository 创建分块仓储
func NewChunkRepository(db *gorm.DB) ChunkRepository {
	return &chunkRepository{db: db}
}

// CreateBatch 批量创建分块
func (r *chunkRepository) CreateBatch(chunks []*entity.Chunk) error {
	if len(chunks) == 0 {
		return nil
	}
	return r.db.Create(&chunks).Error
}

// ListByKnowledgeBase 查询知识库下的分块
func (r *chunkRepository) ListByKnowledgeBase(knowledgeBaseID uint, chunkIDs []uint) ([]*entity.Chunk, error) {
	var chunks []*entity.Chunk
	query := r.db.Where("knowledge_base_id = ? AND is_enabled = ?", knowledgeBaseID, true).Order("id ASC")
	if len(chunkIDs) > 0 {
		query = query.Where("id IN ?", chunkIDs)
	}
	if err := query.Find(&chunks).Error; err != nil {
		return nil, err
	}
	return chunks, nil
}

// ListByIDs 根据分块 ID 列表查询
func (r *chunkRepository) ListByIDs(chunkIDs []uint) ([]*entity.Chunk, error) {
	var chunks []*entity.Chunk
	if len(chunkIDs) == 0 {
		return chunks, nil
	}
	if err := r.db.Where("id IN ?", chunkIDs).Find(&chunks).Error; err != nil {
		return nil, err
	}
	return chunks, nil
}

// UpdateStatusByIDs 更新分块状态
func (r *chunkRepository) UpdateStatusByIDs(chunkIDs []uint, status int) error {
	if len(chunkIDs) == 0 {
		return nil
	}
	return r.db.Model(&entity.Chunk{}).Where("id IN ?", chunkIDs).Update("chunk_status", status).Error
}
