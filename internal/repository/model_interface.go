package repository

import "docmind/internal/model/entity"

// ModelRepository 模型配置仓储接口
type ModelRepository interface {
	Create(model *entity.Model) error
	Update(model *entity.Model) error
	Delete(id uint) error
	FindByID(id uint) (*entity.Model, error)
	FindByName(name string) (*entity.Model, error)
	List(modelType string) ([]*entity.Model, error)
}
