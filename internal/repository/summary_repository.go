package repository

import (
	"errors"

	"docmind/internal/model/entity"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type summaryRepository struct {
	db *gorm.DB
}

// NewSummaryRepository 创建会话摘要仓储
func NewSummaryRepository(db *gorm.DB) SummaryRepository {
	return &summaryRepository{db: db}
}

func (r *summaryRepository) GetBySession(sessionID uint) (*entity.SessionSummary, error) {
	var summary entity.SessionSummary
	err := r.db.Where("session_id = ?", sessionID).First(&summary).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &summary, nil
}

func (r *summaryRepository) Upsert(summary *entity.SessionSummary) error {
	// 每个会话至多一行：按 session_id 冲突时全量更新
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "session_id"}},
		UpdateAll: true,
	}).Create(summary).Error
}

func (r *summaryRepository) DeleteBySession(sessionID uint) error {
	return r.db.Where("session_id = ?", sessionID).Delete(&entity.SessionSummary{}).Error
}
