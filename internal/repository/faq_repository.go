package repository

import (
	"errors"
	"strings"

	"docmind/internal/model/entity"

	"gorm.io/gorm"
)

type faqRepository struct {
	db *gorm.DB
}

func NewFAQRepository(db *gorm.DB) FAQRepository {
	return &faqRepository{db: db}
}

func (r *faqRepository) Create(item *entity.FAQ) error {
	return r.db.Create(item).Error
}

func (r *faqRepository) BatchCreate(items []*entity.FAQ) error {
	if len(items) == 0 {
		return nil
	}
	return r.db.Create(&items).Error
}

func (r *faqRepository) FindByID(id uint) (*entity.FAQ, error) {
	var item entity.FAQ
	err := r.db.First(&item, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *faqRepository) List(filter FAQListFilter) ([]*entity.FAQ, int64, error) {
	query := r.db.Model(&entity.FAQ{}).Where("knowledge_base_id = ?", filter.KnowledgeBaseID)
	if filter.TagID != nil {
		query = query.Where("tag_id = ?", *filter.TagID)
	}
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		query = query.Where("standard_question ILIKE ? OR answer ILIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []*entity.FAQ
	err := query.Order("sort_order ASC").Order("updated_at DESC").Offset(filter.Offset).Limit(filter.Limit).Find(&items).Error
	return items, total, err
}

func (r *faqRepository) Update(item *entity.FAQ) error {
	return r.db.Save(item).Error
}

func (r *faqRepository) DeleteBatch(ids []uint) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.Where("id IN ?", ids).Delete(&entity.FAQ{}).Error
}

func (r *faqRepository) DeleteByKnowledgeBase(knowledgeBaseID uint) error {
	return r.db.Where("knowledge_base_id = ?", knowledgeBaseID).Delete(&entity.FAQ{}).Error
}

func (r *faqRepository) CountByTagIDs(knowledgeBaseID uint, tagIDs []uint) (map[uint]int64, error) {
	result := make(map[uint]int64, len(tagIDs))
	if len(tagIDs) == 0 {
		return result, nil
	}
	type statRow struct {
		TagID uint
		Count int64
	}
	var rows []statRow
	err := r.db.Model(&entity.FAQ{}).
		Select("tag_id, COUNT(*) as count").
		Where("knowledge_base_id = ? AND tag_id IN ?", knowledgeBaseID, tagIDs).
		Group("tag_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.TagID] = row.Count
	}
	return result, nil
}
