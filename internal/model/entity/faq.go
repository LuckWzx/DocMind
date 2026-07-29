package entity

import "database/sql/driver"

// FAQStringList FAQ 相关的字符串列表字段
type FAQStringList []string

func (l *FAQStringList) Scan(value interface{}) error {
	return scanJSONStruct(value, l)
}

func (l FAQStringList) Value() (driver.Value, error) {
	return jsonStructValue(l)
}

// FAQ FAQ 问答对实体
type FAQ struct {
	BaseEntity
	UserID            uint          `gorm:"column:user_id;not null;index" json:"user_id"`
	KnowledgeBaseID   uint          `gorm:"column:knowledge_base_id;not null;index" json:"knowledge_base_id"`
	StandardQuestion  string        `gorm:"type:varchar(500);not null;column:standard_question" json:"standard_question"`
	Answer            string        `gorm:"type:text;not null;column:answer" json:"answer"`
	SimilarQuestions  FAQStringList `gorm:"column:similar_questions;type:json" json:"similar_questions"`
	NegativeQuestions FAQStringList `gorm:"column:negative_questions;type:json" json:"negative_questions"`
	TagID             *uint         `gorm:"column:tag_id;index" json:"tag_id"`
	IsEnabled         bool          `gorm:"column:is_enabled;default:true" json:"is_enabled"`
	IsRecommended     bool          `gorm:"column:is_recommended;default:false" json:"is_recommended"`
	SortOrder         int           `gorm:"type:int;default:0;column:sort_order" json:"sort_order"`
}

// TableName 指定表名
func (FAQ) TableName() string {
	return "faqs"
}
