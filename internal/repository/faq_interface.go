package repository

import "docmind/internal/model/entity"

type FAQListFilter struct {
	KnowledgeBaseID uint
	TagID           *uint
	Keyword         string
	Offset          int
	Limit           int
}

type FAQRepository interface {
	Create(item *entity.FAQ) error
	BatchCreate(items []*entity.FAQ) error
	FindByID(id uint) (*entity.FAQ, error)
	List(filter FAQListFilter) ([]*entity.FAQ, int64, error)
	Update(item *entity.FAQ) error
	DeleteBatch(ids []uint) error
	DeleteByKnowledgeBase(knowledgeBaseID uint) error
	CountByTagIDs(knowledgeBaseID uint, tagIDs []uint) (map[uint]int64, error)
}
