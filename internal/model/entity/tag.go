package entity

// Tag 标签，用于给 Knowledge 和 Chunk 分类
type Tag struct {
	BaseEntity
	Name            string `gorm:"type:varchar(255);not null;comment:标签名称" json:"name"`
	Color           string `gorm:"type:varchar(32);comment:标签颜色" json:"color"`
	KnowledgeBaseID uint   `gorm:"index;comment:所属知识库ID" json:"knowledge_base_id"`
	SortOrder       int    `gorm:"default:0;comment:排序权重" json:"sort_order"`
}

// TableName 指定表名
func (Tag) TableName() string {
	return "tags"
}
