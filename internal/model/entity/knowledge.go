package entity

// Knowledge 知识条目，知识库中的一条知识（一份文档或一组 FAQ）
type Knowledge struct {
	BaseEntity
	Title           string `gorm:"type:varchar(255);comment:条目标题" json:"title"`
	Description     string `gorm:"type:text;comment:条目描述" json:"description"`
	Type            string `gorm:"type:varchar(32);default:'manual';comment:类型 manual/faq" json:"type"`
	ParseStatus     string `gorm:"type:varchar(32);default:'pending';comment:解析状态" json:"parse_status"`
	KnowledgeBaseID uint   `gorm:"index;comment:所属知识库ID" json:"knowledge_base_id"`
	FileURL         string `gorm:"type:text;comment:原始文件URL或本地存储路径" json:"file_url"`
	FileType        string `gorm:"type:varchar(32);comment:文件类型" json:"file_type"`
	TagID           uint   `gorm:"index;comment:标签ID" json:"tag_id"`
	ErrorMessage    string `gorm:"type:text;comment:错误信息" json:"error_message"`
}

// TableName 指定表名
func (Knowledge) TableName() string {
	return "knowledges"
}
