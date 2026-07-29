package repository

import "docmind/internal/model/entity"

// KnowledgeBaseRepository 知识库仓储接口
type KnowledgeBaseRepository interface {
	FindByID(id uint) (*entity.KnowledgeBase, error)
}
