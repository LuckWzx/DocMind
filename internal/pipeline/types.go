package pipeline

import (
	"context"
	"time"

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

// PipelineReranker Rerank 重排序接口，由 service 层实现
type PipelineReranker interface {
	Rerank(ctx context.Context, query string, documents []string, topK int) ([]PipelineRerankResult, error)
}

// PipelineRerankerFactory Rerank 工厂接口，根据模型 ID 创建 Reranker 实例
type PipelineRerankerFactory interface {
	CreateReranker(ctx context.Context, modelID string) (PipelineReranker, error)
}

// PipelineRerankResult Rerank 单条结果
type PipelineRerankResult struct {
	Index          int     // 原始文档在输入列表中的下标
	RelevanceScore float64 // 相关度分数（0~1）
}

// PipelineKeywordDriver BM25 关键词检索驱动接口，由 service 层实现（基于 pg_search）
type PipelineKeywordDriver interface {
	EnsureIndex(ctx context.Context) error
	Search(ctx context.Context, params PipelineKeywordSearchParams) ([]SearchResult, error)
}

// PipelineKeywordSearchParams BM25 关键词检索参数
type PipelineKeywordSearchParams struct {
	KnowledgeBaseIDs []uint  // 知识库过滤（已按用户隔离）
	Query            string  // 检索文本（改写后的查询）
	TopK             int     // 返回条数
	Threshold        float64 // BM25 分数阈值，<=0 不过滤
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
	ChunkID        uint
	KnowledgeID    uint
	Content        string
	KnowledgeTitle string
	Score          float64
}

// PipelineDeps Pipeline 外部依赖，在 NewPipeline 时注入
type PipelineDeps struct {
	EmbedderFactory PipelineEmbedderFactory
	RerankerFactory PipelineRerankerFactory
	KeywordSearch   PipelineKeywordDriver
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

// StepInfo 步骤信息（用于实时回调）
type StepInfo struct {
	StepName   string      // 步骤名称（query_understand, knowledge_search 等）
	StartTime  time.Time   // 步骤开始时间
	EndTime    time.Time   // 步骤结束时间
	Duration   int64       // 步骤耗时（毫秒）
	ToolCallID string      // 工具调用 ID
	Success    bool        // 是否成功
	Data       interface{} // 步骤数据（可选）
}

// StepCallback 步骤回调函数类型
// 在每个步骤开始和结束时调用，用于实时发送进度事件
type StepCallback func(step StepInfo)

// Context RAG Pipeline 上下文，在所有节点间传递
type Context struct {
	// 输入
	Query           string
	SessionID       uint
	UserID          uint
	AgentConfig     *AgentConfig      // 从 Agent 动态解析的配置
	HistoryMessages []*schema.Message // 对话历史（由调用方加载）

	// 依赖
	ModelRepo    repository.ModelRepository
	PipelineDeps *PipelineDeps // 外部依赖（向量检索等）

	// 步骤回调（由调用方注入，用于实时发送进度事件）
	StepCallback StepCallback

	// 中间结果
	RewrittenQuery  string            // 改写后的查询
	Intent          string            // 意图分类结果
	SearchResults   []SearchResult    // 向量检索结果
	KeywordResults  []SearchResult    // BM25 关键词检索结果（供 RRF 融合使用）
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
	// 多轮对话配置
	MultiTurnEnabled bool // 是否启用多轮对话
	// HistoryTurns 压缩时保底保留的最近完整轮数（原文不压缩，直接随摘要一起发给模型）
	HistoryTurns int

	// 查询改写配置
	QueryUnderstandModelID string // 用于问题改写的模型ID（可选，为空则复用 ModelID）
	RewritePromptSystem    string // 改写系统提示词（为空使用默认）
	RewritePromptUser      string // 改写用户提示词模板（为空使用默认）
	EmbeddingTopK          int
	VectorThreshold        float64
	KeywordTopK            int     // BM25 检索返回条数
	KeywordThreshold       float64 // BM25 分数阈值（非 0~1 量纲，<=0 不过滤）
	RerankTopK             int
	RerankThreshold        float64 // Rerank 相关度阈值，低于此值的结果将被过滤（0~1）
}

// SearchResult 搜索结果
type SearchResult struct {
	ChunkID        uint
	Content        string
	Score          float64
	KnowledgeID    uint
	KnowledgeTitle string
}
