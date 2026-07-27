package entity

// ============================================================================
// Chunk 配置依赖
// ============================================================================

// ImageProcessingConfig 图片处理配置
type ImageProcessingConfig struct {
	EnableOCR     bool `json:"enable_ocr"`
	EnableCaption bool `json:"enable_caption"`
}

// FAQChunkMetadata FAQ 条目的元数据，存入 Chunk.Metadata 列
type FAQChunkMetadata struct {
	StandardQuestion  string   `json:"standard_question"`
	SimilarQuestions  []string `json:"similar_questions,omitempty"`
	NegativeQuestions []string `json:"negative_questions,omitempty"`
	Answers           []string `json:"answers,omitempty"`
	AnswerStrategy    string   `json:"answer_strategy,omitempty"` // all / random
	Version           int      `json:"version,omitempty"`
	Source            string   `json:"source,omitempty"` // import / manual
}

// Chunk 文本分块，知识被拆分后的最小检索单元
type Chunk struct {
	BaseEntity
	Content         string `gorm:"type:text;comment:分块文本内容" json:"content"`
	ChunkIndex      int    `gorm:"index;comment:文档中的顺序号" json:"chunk_index"`
	KnowledgeID     uint   `gorm:"index;comment:所属知识条目ID" json:"knowledge_id"`
	KnowledgeBaseID uint   `gorm:"index;comment:所属知识库ID" json:"knowledge_base_id"`
	ChunkType       string `gorm:"type:varchar(32);default:'text';comment:分块类型" json:"chunk_type"`
	ChunkStatus     int    `gorm:"default:0;comment:分块状态 0=默认 1=已存储 2=已索引" json:"chunk_status"`
	ParentChunkID   uint   `gorm:"index;comment:父分块ID" json:"parent_chunk_id"`
	TagID           uint   `gorm:"index;comment:标签ID" json:"tag_id"`
	Metadata        JSON   `gorm:"type:json;comment:扩展元数据" json:"metadata"`
	ContentHash     string `gorm:"type:varchar(64);comment:内容hash 去重/增量更新" json:"content_hash"`
	IsEnabled       bool   `gorm:"default:true;comment:是否启用" json:"is_enabled"`
}

// TableName 指定表名
func (Chunk) TableName() string {
	return "chunks"
}
