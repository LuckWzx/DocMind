package repository

import "docmind/internal/model/entity"

type KnowledgeListFilter struct {
	KnowledgeBaseID uint
	TagIDs          []uint
	Keyword         string
	FileType        string
	ParseStatus     string
	Source          string
	Offset          int
	Limit           int
}

type KnowledgeRepository interface {
	Create(item *entity.Knowledge) error
	FindByID(id uint) (*entity.Knowledge, error)
	List(filter KnowledgeListFilter) ([]*entity.Knowledge, int64, error)
	Update(item *entity.Knowledge) error
	Delete(id uint) error
	DeleteByKnowledgeBase(knowledgeBaseID uint) error
}
