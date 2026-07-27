package entity

import "time"

// FAQ FAQ 问答对实体
type FAQ struct {
	BaseEntity
	UserID           uint       `gorm:"column:user_id;not null;index" json:"user_id"`
	KnowledgeBaseID  uint       `gorm:"column:knowledge_base_id;not null;index" json:"knowledge_base_id"`
	Question         string     `gorm:"type:varchar(500);not null;column:question" json:"question"`
	Answer           string     `gorm:"type:text;not null;column:answer" json:"answer"`
	SimilarQuestions []string   `gorm:"column:similar_questions;type:json" json:"similar_questions"`
	SortOrder        int        `gorm:"type:int;default:0;column:sort_order" json:"sort_order"`
	Status           int        `gorm:"type:smallint;default:1;column:status" json:"status"` // 1:启用, 2:禁用
	DeletedAt        *time.Time `gorm:"column:deleted_at;index" json:"deleted_at"`
}

// TableName 指定表名
func (FAQ) TableName() string {
	return "faqs"
}
