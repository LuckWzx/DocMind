package repository

import (
	"errors"
	"strings"

	"docmind/internal/model/entity"

	"gorm.io/gorm"
)

type knowledgeRepository struct {
	db *gorm.DB
}

func NewKnowledgeRepository(db *gorm.DB) KnowledgeRepository {
	return &knowledgeRepository{db: db}
}

func (r *knowledgeRepository) Create(item *entity.Knowledge) error {
	return r.db.Create(item).Error
}

func (r *knowledgeRepository) FindByID(id uint) (*entity.Knowledge, error) {
	var item entity.Knowledge
	err := r.db.First(&item, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *knowledgeRepository) FindByIDs(ids []uint) ([]*entity.Knowledge, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var items []*entity.Knowledge
	if err := r.db.Where("id IN ?", ids).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *knowledgeRepository) List(filter KnowledgeListFilter) ([]*entity.Knowledge, int64, error) {
	query := r.db.Model(&entity.Knowledge{}).Where("knowledge_base_id = ?", filter.KnowledgeBaseID)
	if len(filter.TagIDs) > 0 {
		query = query.Where("tag_id IN ?", filter.TagIDs)
	}
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		query = query.Where("title ILIKE ? OR file_name ILIKE ? OR description ILIKE ?", "%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
	}
	if filter.FileType != "" {
		query = query.Where("file_type = ?", filter.FileType)
	}
	if filter.ParseStatus != "" {
		query = query.Where("parse_status = ?", filter.ParseStatus)
	}
	if filter.Source != "" {
		query = query.Where("source = ?", filter.Source)
	}
	if !filter.StartTime.IsZero() {
		query = query.Where("updated_at >= ?", filter.StartTime)
	}
	if !filter.EndTime.IsZero() {
		query = query.Where("updated_at <= ?", filter.EndTime)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []*entity.Knowledge
	err := query.Order("updated_at DESC").Offset(filter.Offset).Limit(filter.Limit).Find(&items).Error
	return items, total, err
}

func (r *knowledgeRepository) Update(item *entity.Knowledge) error {
	return r.db.Save(item).Error
}

func (r *knowledgeRepository) Delete(id uint) error {
	return r.db.Delete(&entity.Knowledge{}, id).Error
}

func (r *knowledgeRepository) DeleteByKnowledgeBase(knowledgeBaseID uint) error {
	return r.db.Where("knowledge_base_id = ?", knowledgeBaseID).Delete(&entity.Knowledge{}).Error
}
