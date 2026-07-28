package repository

import (
	"errors"

	"docmind/internal/model/entity"

	"gorm.io/gorm"
)

type systemSettingRepository struct {
	db *gorm.DB
}

// NewSystemSettingRepository 创建系统配置仓储
func NewSystemSettingRepository(db *gorm.DB) SystemSettingRepository {
	return &systemSettingRepository{db: db}
}

func (r *systemSettingRepository) FindByKey(key string) (*entity.SystemSetting, error) {
	var setting entity.SystemSetting
	err := r.db.Where("key = ?", key).First(&setting).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &setting, nil
}

func (r *systemSettingRepository) Upsert(setting *entity.SystemSetting) error {
	existing, err := r.FindByKey(setting.Key)
	if err != nil {
		return err
	}
	if existing == nil {
		return r.db.Create(setting).Error
	}
	existing.Value = setting.Value
	return r.db.Save(existing).Error
}
