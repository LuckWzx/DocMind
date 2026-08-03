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
	// FindDuplicate 按 名称+类型+来源+供应商 查找重复配置（四项一致才视为重复）
	FindDuplicate(userID uint, name, modelType, source, provider string) (*entity.Model, error)
	List(modelType string, userID uint) ([]*entity.Model, error)
	ListAll(modelType string) ([]*entity.Model, error)
}
