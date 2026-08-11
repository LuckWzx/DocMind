package entity

import "time"

// BaseEntity 基础实体（自增主键 + 时间戳，硬删除语义）
type BaseEntity struct {
	ID        uint      `gorm:"primaryKey;autoIncrement;comment:主键ID" json:"id"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime;comment:创建时间" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime;comment:更新时间" json:"updated_at"`
}
