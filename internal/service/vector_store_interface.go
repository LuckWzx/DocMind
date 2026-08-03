package service

import (
	"context"

	"docmind/internal/model/dto/request"
	dto "docmind/internal/model/dto/response"
	"docmind/internal/model/entity"
	"docmind/pkg/response"
)

// VectorItem 待写入向量项
type VectorItem struct {
	UserID          uint
	VectorStoreID   uint
	KnowledgeBaseID uint
	KnowledgeID     uint
	ChunkID         uint
	Embedding       []float32
	ContentHash     string
}

// VectorSearchParams 向量检索参数
type VectorSearchParams struct {
	UserID           uint
	VectorStoreID    uint
	KnowledgeBaseIDs []uint
	KnowledgeIDs     []uint
	QueryVector      []float32
	TopK             int
	Threshold        float64
	ExcludeChunkIDs  []uint
}

// VectorSearchResult 向量检索结果
type VectorSearchResult struct {
	ChunkID         uint    `json:"chunk_id"`
	KnowledgeID     uint    `json:"knowledge_id"`
	KnowledgeBaseID uint    `json:"knowledge_base_id"`
	Content         string  `json:"content"`
	KnowledgeTitle  string  `json:"knowledge_title"`
	Score           float64 `json:"score"`
}

// VectorDriver 向量引擎驱动接口
type VectorDriver interface {
	UpsertChunks(ctx context.Context, items []VectorItem) error
	DeleteByChunkIDs(ctx context.Context, chunkIDs []uint) error
	Search(ctx context.Context, params VectorSearchParams) ([]VectorSearchResult, error)
}

// VectorStoreService 向量存储服务接口
type VectorStoreService interface {
	Create(userID uint, req *request.CreateVectorStoreRequest) (*dto.VectorStoreResponse, error)
	GetByID(userID, id uint) (*dto.VectorStoreResponse, error)
	Update(userID, id uint, req *request.UpdateVectorStoreRequest) (*dto.VectorStoreResponse, error)
	Delete(userID, id uint) error
	List(userID uint, req *request.VectorStoreListRequest) (*response.PageResponse, error)
	TestConnection(userID, id uint) error
	Search(ctx context.Context, userID, id uint, req *request.VectorSearchRequest) ([]*dto.VectorSearchResultResponse, error)
	IndexKnowledgeBase(ctx context.Context, userID, id, knowledgeBaseID uint, req *request.IndexKnowledgeBaseRequest) (*dto.IndexKnowledgeBaseResponse, error)
	GetEntityByID(userID, id uint) (*entity.VectorStore, error)
}
