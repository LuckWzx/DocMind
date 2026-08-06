package response

import "time"

// VectorStoreResponse 向量存储响应
type VectorStoreResponse struct {
	ID               uint      `json:"id"`
	UserID           uint      `json:"user_id"`
	Name             string    `json:"name"`
	EngineType       string    `json:"engine_type"`
	ConnectionConfig string    `json:"connection_config"`
	IndexConfig      string    `json:"index_config"`
	Status           int       `json:"status"`
	Source           string    `json:"source"`
	Readonly         bool      `json:"readonly"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// VectorSearchResultResponse 向量检索结果响应
type VectorSearchResultResponse struct {
	ChunkID         uint    `json:"chunk_id"`
	KnowledgeID     uint    `json:"knowledge_id"`
	KnowledgeBaseID uint    `json:"knowledge_base_id"`
	Content         string  `json:"content"`
	Score           float64 `json:"score"`
}

// IndexKnowledgeBaseResponse 知识库索引响应
type IndexKnowledgeBaseResponse struct {
	KnowledgeBaseID uint `json:"knowledge_base_id"`
	IndexedCount    int  `json:"indexed_count"`
}
