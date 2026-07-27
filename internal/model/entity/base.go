package entity

import (
	"time"

	"gorm.io/gorm"
)

// BaseEntity 基础实体（自增主键 + 软删除）
type BaseEntity struct {
	ID        uint           `gorm:"primaryKey;autoIncrement;comment:主键ID" json:"id"`
	CreatedAt time.Time      `gorm:"column:created_at;autoCreateTime;comment:创建时间" json:"created_at"`
	UpdatedAt time.Time      `gorm:"column:updated_at;autoUpdateTime;comment:更新时间" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;index;comment:软删除时间" json:"deleted_at,omitempty"`
}
