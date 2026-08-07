package repository

import (
	"docmind/internal/model/entity"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type modelContextWindowMissingRepository struct {
	db *gorm.DB
}

// NewModelContextWindowMissingRepository 创建上下文大小缺失记录仓储
func NewModelContextWindowMissingRepository(db *gorm.DB) ModelContextWindowMissingRepository {
	return &modelContextWindowMissingRepository{db: db}
}

func (r *modelContextWindowMissingRepository) Upsert(record *entity.ModelContextWindowMissing) error {
	// 每个模型至多一行：按 model_id 冲突时全量更新
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "model_id"}},
		UpdateAll: true,
	}).Create(record).Error
}

func (r *modelContextWindowMissingRepository) ClearByModelID(modelID uint) error {
	// 硬删除：释放 model_id 唯一索引，允许后续同一模型再次写入缺失记录
	return r.db.Unscoped().Where("model_id = ?", modelID).Delete(&entity.ModelContextWindowMissing{}).Error
}

func (r *modelContextWindowMissingRepository) ListAll() ([]*entity.ModelContextWindowMissing, error) {
	var records []*entity.ModelContextWindowMissing
	err := r.db.Order("created_at DESC").Find(&records).Error
	if err != nil {
		return nil, err
	}
	return records, nil
}
