package repository

import "docmind/internal/model/entity"

// KnowledgeRepository 知识条目仓储接口
type KnowledgeRepository interface {
	Create(knowledge *entity.Knowledge) error
	Update(knowledge *entity.Knowledge) error
	FindByID(id uint) (*entity.Knowledge, error)
}
