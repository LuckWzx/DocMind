package repository

import (
	"errors"
	"strings"

	"docmind/internal/model/entity"

	"gorm.io/gorm"
)

type tagRepository struct {
	db *gorm.DB
}

func NewTagRepository(db *gorm.DB) TagRepository {
	return &tagRepository{db: db}
}

func (r *tagRepository) Create(item *entity.Tag) error {
	return r.db.Create(item).Error
}

func (r *tagRepository) FindByID(id uint) (*entity.Tag, error) {
	var item entity.Tag
	err := r.db.First(&item, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *tagRepository) ListByKnowledgeBase(knowledgeBaseID uint, keyword string) ([]*entity.Tag, error) {
	query := r.db.Where("knowledge_base_id = ?", knowledgeBaseID)
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		query = query.Where("name ILIKE ?", "%"+keyword+"%")
	}
	var items []*entity.Tag
	err := query.Order("sort_order ASC").Order("created_at ASC").Find(&items).Error
	return items, err
}

func (r *tagRepository) Update(item *entity.Tag) error {
	return r.db.Save(item).Error
}

func (r *tagRepository) Delete(id uint) error {
	return r.db.Delete(&entity.Tag{}, id).Error
}
