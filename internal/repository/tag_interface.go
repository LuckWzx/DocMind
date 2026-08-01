package repository

import "docmind/internal/model/entity"

type TagRepository interface {
	Create(item *entity.Tag) error
	FindByID(id uint) (*entity.Tag, error)
	FindByNameAndKB(name string, knowledgeBaseID uint) (*entity.Tag, error)
	ListByKnowledgeBase(knowledgeBaseID uint, keyword string, offset, limit int) ([]*entity.Tag, int64, error)
	Update(item *entity.Tag) error
	Delete(id uint) error
}
