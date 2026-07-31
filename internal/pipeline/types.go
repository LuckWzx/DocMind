package pipeline

import (
	"context"

	"docmind/internal/repository"

	"github.com/cloudwego/eino/schema"
)

// ===== 依赖注入接口（避免 pipeline → service 循环依赖） =====

// PipelineEmbedder 向量嵌入接口，由 service 层实现
type PipelineEmbedder interface {
	EmbedStrings(ctx context.Context, texts []string) ([]float64, error)
}

// PipelineVectorDriver 向量检索驱动接口，由 service 层实现
type PipelineVectorDriver interface {
	EnsureSchema(ctx context.Context) error
	Search(ctx context.Context, params PipelineVectorSearchParams) ([]PipelineVectorSearchResult, error)
}

// PipelineVectorSearchParams 向量检索参数
type PipelineVectorSearchParams struct {
	UserID           uint
	VectorStoreID    uint
	KnowledgeBaseIDs []uint
	QueryVector      []float32
	TopK             int
	Threshold        float64
}

// PipelineVectorSearchResult 向量检索结果
type PipelineVectorSearchResult struct {
	ChunkID     uint
	KnowledgeID uint
	Score       float64
}

// PipelineDeps Pipeline 外部依赖，在 NewPipeline 时注入
type PipelineDeps struct {
	EmbedderFactory PipelineEmbedderFactory
	KBRepo          repository.KnowledgeBaseRepository
	VectorStoreRepo repository.VectorStoreRepository
	PrimaryDB       interface{} // *gorm.DB，使用 interface 避免导入 gorm
	CreateDriver    func(store interface{}) (PipelineVectorDriver, func(), error)
}

// PipelineEmbedderFactory 嵌入模型工厂接口
type PipelineEmbedderFactory interface {
	CreateEmbedder(ctx context.Context, modelID string) (PipelineEmbedder, error)
}

// ===== Pipeline 上下文 =====

// Context RAG Pipeline 上下文，在所有节点间传递
type Context struct {
	// 输入
	Query       string
	SessionID   uint
	UserID      uint
	AgentConfig *AgentConfig // 从 Agent 动态解析的配置

	// 依赖
	ModelRepo    repository.ModelRepository
	PipelineDeps *PipelineDeps // 外部依赖（向量检索等）

	// 中间结果
	RewrittenQuery  string            // 改写后的查询
	Intent          string            // 意图分类结果
	SearchResults   []SearchResult    // 向量检索结果
	RerankedResults []SearchResult    // 重排序后的结果
	Messages        []*schema.Message // 拼接后的 Prompt

	// 输出
	Stream *schema.StreamReader[*schema.Message]
}

// AgentConfig Agent 配置（从数据库动态解析）
type AgentConfig struct {
	ModelID            string
	RerankModelID      string
	EmbeddingModelID   string
	KnowledgeBaseIDs   []string
	Temperature        float64
	MaxTokens          int
	SystemPrompt       string
	EnableQueryRewrite bool
	EmbeddingTopK      int
	VectorThreshold    float64
	RerankTopK         int
}

// SearchResult 搜索结果
type SearchResult struct {
	ChunkID        uint
	Content        string
	Score          float64
	KnowledgeID    uint
	KnowledgeTitle string
}
