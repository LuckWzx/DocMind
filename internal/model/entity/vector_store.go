package entity

import "time"

// VectorStore 向量存储实例
type VectorStore struct {
	BaseEntity
	UserID           uint       `gorm:"column:user_id;not null;index" json:"user_id"`
	Name             string     `gorm:"type:varchar(255);not null;column:name" json:"name"`
	EngineType       string     `gorm:"type:varchar(50);not null;column:engine_type" json:"engine_type"`
	ConnectionConfig string     `gorm:"type:json;column:connection_config" json:"connection_config"`
	IndexConfig      string     `gorm:"type:json;column:index_config" json:"index_config"`
	Status           int        `gorm:"type:smallint;default:1;column:status" json:"status"` // 1:正常, 2:禁用
	DeletedAt        *time.Time `gorm:"column:deleted_at;index" json:"deleted_at"`
}

// TableName 指定表名
func (VectorStore) TableName() string {
	return "vector_stores"
}
