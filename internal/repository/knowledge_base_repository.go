package repository

import (
	"errors"

	"docmind/internal/model/entity"

	"gorm.io/gorm"
)

type knowledgeBaseRepository struct {
	db *gorm.DB
}

// NewKnowledgeBaseRepository 创建知识库仓储
func NewKnowledgeBaseRepository(db *gorm.DB) KnowledgeBaseRepository {
	return &knowledgeBaseRepository{db: db}
}

func (r *knowledgeBaseRepository) Create(kb *entity.KnowledgeBase) error {
	return r.db.Create(kb).Error
}

func (r *knowledgeBaseRepository) Update(kb *entity.KnowledgeBase) error {
	return r.db.Save(kb).Error
}

func (r *knowledgeBaseRepository) Delete(id uint) error {
	// 硬删除：与知识库删除链路一致，避免软删残留数据堆积
	return r.db.Unscoped().Delete(&entity.KnowledgeBase{}, id).Error
}

// FindByID 根据 ID 获取知识库
func (r *knowledgeBaseRepository) FindByID(id uint) (*entity.KnowledgeBase, error) {
	var kb entity.KnowledgeBase
	err := r.db.First(&kb, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &kb, nil
}

func (r *knowledgeBaseRepository) FindByUserID(userID, id uint) (*entity.KnowledgeBase, error) {
	var kb entity.KnowledgeBase
	err := r.db.Where("id = ? AND user_id = ?", id, userID).First(&kb).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &kb, nil
}

func (r *knowledgeBaseRepository) ListByUser(userID uint) ([]*entity.KnowledgeBase, error) {
	var items []*entity.KnowledgeBase
	err := r.db.Where("user_id = ?", userID).
		Order("is_pinned DESC").Order("updated_at DESC").Find(&items).Error
	return items, err
}
