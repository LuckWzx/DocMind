package repository

import "docmind/internal/model/entity"

// KnowledgeBaseRepository 知识库仓储接口
type KnowledgeBaseRepository interface {
	Create(kb *entity.KnowledgeBase) error
	Update(kb *entity.KnowledgeBase) error
	Delete(id uint) error
	FindByID(id uint) (*entity.KnowledgeBase, error)
	FindByUserID(userID, id uint) (*entity.KnowledgeBase, error)
	ListByUser(userID uint) ([]*entity.KnowledgeBase, error)
}
