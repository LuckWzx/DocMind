package request

import "docmind/pkg/response"

// VectorStoreListRequest 向量存储列表请求
type VectorStoreListRequest struct {
	response.PageRequest
}

// CreateVectorStoreRequest 创建向量存储请求
type CreateVectorStoreRequest struct {
	Name             string `json:"name" binding:"required,min=1,max=255"`
	EngineType       string `json:"engine_type" binding:"required,oneof=postgres qdrant milvus weaviate elasticsearch"`
	ConnectionConfig string `json:"connection_config"`
	IndexConfig      string `json:"index_config"`
}

// UpdateVectorStoreRequest 更新向量存储请求
type UpdateVectorStoreRequest struct {
	Name             string `json:"name" binding:"omitempty,min=1,max=255"`
	EngineType       string `json:"engine_type" binding:"omitempty,oneof=postgres qdrant milvus weaviate elasticsearch"`
	ConnectionConfig string `json:"connection_config"`
	IndexConfig      string `json:"index_config"`
	Status           *int   `json:"status" binding:"omitempty,oneof=1 2"`
}
