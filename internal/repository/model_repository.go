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

func (r *modelRepository) FindByID(id uint) (*entity.Model, error) {
	var model entity.Model
	err := r.db.First(&model, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &model, nil
}

func (r *modelRepository) FindByName(name string) (*entity.Model, error) {
	var model entity.Model
	err := r.db.Where("name = ?", name).First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &model, nil
}

func (r *modelRepository) List(modelType string) ([]*entity.Model, error) {
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
