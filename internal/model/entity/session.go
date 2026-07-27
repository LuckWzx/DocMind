package entity

// Session 会话，一次连续的问答对话
type Session struct {
	BaseEntity
	Title           string `gorm:"type:varchar(255);comment:会话标题" json:"title"`
	Description     string `gorm:"type:text;comment:会话描述" json:"description"`
	KnowledgeBaseID uint   `gorm:"index;comment:绑定的知识库ID" json:"knowledge_base_id"`

	// 关联（不存入数据库）
	Messages []Message `gorm:"foreignKey:SessionID" json:"-"`
}

// TableName 指定表名
func (Session) TableName() string {
	return "sessions"
}
