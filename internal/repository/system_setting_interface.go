package repository

import "docmind/internal/model/entity"

// SystemSettingRepository 系统配置仓储接口
type SystemSettingRepository interface {
	FindByKey(key string) (*entity.SystemSetting, error)
	Upsert(setting *entity.SystemSetting) error
}
