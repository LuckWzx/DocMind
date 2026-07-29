package repository

import (
	"errors"

	"docmind/internal/model/entity"

	"gorm.io/gorm"
)

type knowledgeRepository struct {
	db *gorm.DB
}

// NewKnowledgeRepository 创建知识条目仓储
func NewKnowledgeRepository(db *gorm.DB) KnowledgeRepository {
	return &knowledgeRepository{db: db}
}

// Create 创建知识条目
func (r *knowledgeRepository) Create(knowledge *entity.Knowledge) error {
	return r.db.Create(knowledge).Error
}

// Update 更新知识条目
func (r *knowledgeRepository) Update(knowledge *entity.Knowledge) error {
	return r.db.Save(knowledge).Error
}

// FindByID 根据 ID 查询知识条目
func (r *knowledgeRepository) FindByID(id uint) (*entity.Knowledge, error) {
	var knowledge entity.Knowledge
	err := r.db.First(&knowledge, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &knowledge, nil
}
