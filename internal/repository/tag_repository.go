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

func (r *tagRepository) FindByNameAndKB(name string, knowledgeBaseID uint) (*entity.Tag, error) {
	var item entity.Tag
	err := r.db.Where("name = ? AND knowledge_base_id = ?", name, knowledgeBaseID).First(&item).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *tagRepository) ListByKnowledgeBase(knowledgeBaseID uint, keyword string, offset, limit int) ([]*entity.Tag, int64, error) {
	query := r.db.Model(&entity.Tag{}).Where("knowledge_base_id = ?", knowledgeBaseID)
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		query = query.Where("name ILIKE ?", "%"+keyword+"%")
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []*entity.Tag
	err := query.Order("sort_order ASC").Order("created_at ASC").Offset(offset).Limit(limit).Find(&items).Error
	return items, total, err
}

func (r *tagRepository) Update(item *entity.Tag) error {
	return r.db.Save(item).Error
}

func (r *tagRepository) Delete(id uint) error {
	// 硬删除：与知识库删除链路一致，避免软删残留数据堆积
	return r.db.Unscoped().Delete(&entity.Tag{}, id).Error
}
