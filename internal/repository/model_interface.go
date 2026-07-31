package repository

import "docmind/internal/model/entity"

// ModelRepository 模型配置仓储接口
type ModelRepository interface {
	Create(model *entity.Model) error
	Update(model *entity.Model) error
	Delete(id uint) error
	FindByUserID(id uint, userID uint) (*entity.Model, error)
	FindByID(id uint) (*entity.Model, error)
	FindByName(name string, userID uint) (*entity.Model, error)
	List(modelType string, userID uint) ([]*entity.Model, error)
	ListAll(modelType string) ([]*entity.Model, error)
}
