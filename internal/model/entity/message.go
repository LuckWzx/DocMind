package entity

// ============================================================================
// Message 配置依赖
// ============================================================================

// MessageImages 消息附带的图片列表
type MessageImages []MessageImage

// MessageImage 消息中的单张图片
type MessageImage struct {
	URL     string `json:"url"`               // 图片URL
	Caption string `json:"caption,omitempty"` // 图片描述（OCR/VLM生成的文本）
}

// Reference 引用来源（检索到的 chunk，运行时填充，不存库）
type Reference struct {
	ChunkID        uint    `json:"chunk_id"`
	Content        string  `json:"content"`
	Score          float64 `json:"score"`
	KnowledgeID    uint    `json:"knowledge_id"`
	KnowledgeTitle string  `json:"knowledge_title"`
}

// MentionedItem @提及的知识库/文件
type MentionedItem struct {
	Type string `json:"type"` // knowledge_base / file
	ID   uint   `json:"id"`
	Name string `json:"name,omitempty"`
}

// Message 消息，会话中的一条对话记录
type Message struct {
	BaseEntity
	SessionID           uint          `gorm:"index;comment:所属会话ID" json:"session_id"`
	Role                string        `gorm:"type:varchar(16);not null;comment:角色 user/assistant/system" json:"role"`
	Content             string        `gorm:"type:text;comment:消息正文" json:"content"`
	RenderedContent     string        `gorm:"type:text;comment:渲染给前端的正文" json:"rendered_content"`
	Images              MessageImages `gorm:"type:json;comment:消息附带的图片" json:"images"`
	KnowledgeReferences []Reference   `gorm:"-;comment:引用的知识来源 不存库" json:"knowledge_references"`
	FinishReason        string        `gorm:"type:varchar(32);comment:LLM完成原因" json:"finish_reason"`

	// 关联（不存入数据库）
	Session Session `gorm:"foreignKey:SessionID" json:"-"`
}

// TableName 指定表名
func (Message) TableName() string {
	return "messages"
}
