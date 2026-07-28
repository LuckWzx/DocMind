package entity

// ChunkVector 分块向量索引记录
type ChunkVector struct {
	BaseEntity
	UserID          uint   `gorm:"not null;index;comment:所属用户ID" json:"user_id"`
	VectorStoreID   uint   `gorm:"not null;index;comment:向量存储ID" json:"vector_store_id"`
	KnowledgeBaseID uint   `gorm:"not null;index;comment:知识库ID" json:"knowledge_base_id"`
	KnowledgeID     uint   `gorm:"not null;index;comment:知识条目ID" json:"knowledge_id"`
	ChunkID         uint   `gorm:"not null;uniqueIndex;comment:分块ID" json:"chunk_id"`
	Embedding       string `gorm:"type:text;comment:向量数据占位，后续切换为 pgvector" json:"-"`
	ContentHash     string `gorm:"type:varchar(64);index;comment:内容哈希" json:"content_hash"`
	IsEnabled       bool   `gorm:"default:true;index;comment:是否启用" json:"is_enabled"`
}

// TableName 指定表名
func (ChunkVector) TableName() string {
	return "chunk_vectors"
}
