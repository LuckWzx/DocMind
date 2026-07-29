package repository

import "docmind/internal/model/entity"

type TagRepository interface {
	Create(item *entity.Tag) error
	FindByID(id uint) (*entity.Tag, error)
	ListByKnowledgeBase(knowledgeBaseID uint, keyword string) ([]*entity.Tag, error)
	Update(item *entity.Tag) error
	Delete(id uint) error
}
