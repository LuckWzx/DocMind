package repository

import (
	"errors"

	"docmind/internal/model/entity"

	"gorm.io/gorm"
)

type modelRepository struct {
	db *gorm.DB
}

// NewModelRepository 创建模型仓储
func NewModelRepository(db *gorm.DB) ModelRepository {
	return &modelRepository{db: db}
}

func (r *modelRepository) Create(model *entity.Model) error {
	return r.db.Create(model).Error
}

func (r *modelRepository) Update(model *entity.Model) error {
	return r.db.Save(model).Error
}

func (r *modelRepository) Delete(id uint) error {
	return r.db.Delete(&entity.Model{}, id).Error
}

func (r *modelRepository) FindByUserID(id uint, userID uint) (*entity.Model, error) {
	var model entity.Model
	err := r.db.Where("id = ? AND user_id = ?", id, userID).First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &model, nil
}

func (r *modelRepository) FindByID(id uint) (*entity.Model, error) {
	var model entity.Model
	err := r.db.Where("id = ? ", id).First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &model, nil
}

func (r *modelRepository) FindByName(name string, userID uint) (*entity.Model, error) {
	var model entity.Model
	err := r.db.Where("name = ? AND user_id = ?", name, userID).First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &model, nil
}

// FindDuplicate 按 名称+类型+来源 查询候选后比对供应商，避免 JSON 字段查询的数据库方言差异
func (r *modelRepository) FindDuplicate(userID uint, name, modelType, source, provider string) (*entity.Model, error) {
	var candidates []entity.Model
	err := r.db.Where("name = ? AND user_id = ? AND type = ? AND source = ?", name, userID, modelType, source).Find(&candidates).Error
	if err != nil {
		return nil, err
	}
	for i := range candidates {
		if candidates[i].Parameters.Provider == provider {
			return &candidates[i], nil
		}
	}
	return nil, nil
}

func (r *modelRepository) List(modelType string, userID uint) ([]*entity.Model, error) {
	var models []*entity.Model
	query := r.db.Where("user_id = ?", userID).Order("created_at DESC")
	if modelType != "" {
		query = query.Where("type = ?", modelType)
	}
	if err := query.Find(&models).Error; err != nil {
		return nil, err
	}
	return models, nil
}

// ListAll 查询所有用户的模型（不按 user_id 过滤）
func (r *modelRepository) ListAll(modelType string) ([]*entity.Model, error) {
	var models []*entity.Model
	query := r.db.Order("created_at DESC")
	if modelType != "" {
		query = query.Where("type = ?", modelType)
	}
	if err := query.Find(&models).Error; err != nil {
		return nil, err
	}
	return models, nil
}
