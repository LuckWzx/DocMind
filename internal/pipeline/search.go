package pipeline

import (
	"context"
	"fmt"
	"sort"

	"docmind/internal/model/entity"
)

// SearchKBParams 知识库检索参数（kb_search 工具与 RAG 管道检索段共用）
type SearchKBParams struct {
	UserID           uint
	KnowledgeBaseIDs []string
	Query            string
	EmbeddingTopK    int
	VectorThreshold  float64
	RerankModelID    string
	RerankTopK       int
	RerankThreshold  float64
}

// SearchKB 知识库检索：query → embedding → pgvector → rerank
// 由 node_vector_search / node_rerank 的核心逻辑提取，供 Agent kb_search 工具复用
// （知识库范围为空时返回空结果，全量兜底由调用方决定）
func SearchKB(ctx context.Context, deps *PipelineDeps, params *SearchKBParams) ([]SearchResult, error) {
	if deps == nil || deps.EmbedderFactory == nil || deps.VectorStoreRepo == nil {
		return nil, fmt.Errorf("向量检索: 依赖未注入")
	}
	if len(params.KnowledgeBaseIDs) == 0 {
		return []SearchResult{}, nil
	}
	if params.Query == "" {
		return []SearchResult{}, nil
	}

	// 1. 根据第一个知识库确定 Embedding 模型
	kbID := parseUint(params.KnowledgeBaseIDs[0])
	kb, err := deps.KBRepo.FindByID(kbID)
	if err != nil || kb == nil {
		return nil, fmt.Errorf("向量检索: 知识库不存在: %s", params.KnowledgeBaseIDs[0])
	}

	embedderID := kb.EmbeddingModelID
	if embedderID == "" {
		embedderID = "default"
	}

	// 2. 创建 Embedder 并生成查询向量
	embedder, err := deps.EmbedderFactory.CreateEmbedder(ctx, embedderID)
	if err != nil {
		return nil, fmt.Errorf("向量检索: 创建 Embedder 失败: %w", err)
	}
	vectors, err := embedder.EmbedStrings(ctx, []string{params.Query})
	if err != nil {
		return nil, fmt.Errorf("向量检索: Embedding 调用失败: %w", err)
	}
	if len(vectors) == 0 {
		return nil, fmt.Errorf("向量检索: Embedding 返回空结果")
	}
	queryVector := make([]float32, len(vectors))
	for i, v := range vectors {
		queryVector[i] = float32(v)
	}

	// 3. 查找向量存储：优先知识库配置，回退系统全局默认
	var store *entity.VectorStore
	if kb.VectorStoreID != nil && *kb.VectorStoreID > 0 {
		store, err = deps.VectorStoreRepo.FindByID(*kb.VectorStoreID)
		if err != nil {
			return nil, fmt.Errorf("向量检索: 查询知识库向量存储失败: %w", err)
		}
	}
	if store == nil {
		store, err = deps.VectorStoreRepo.FirstOrCreateGlobalDefault()
		if err != nil {
			return nil, fmt.Errorf("向量检索: 获取默认向量存储失败: %w", err)
		}
	}

	// 4. 创建向量驱动并检索
	driver, cleanup, err := deps.CreateDriver(store)
	if err != nil {
		return nil, fmt.Errorf("向量检索: 创建向量驱动失败: %w", err)
	}
	defer cleanup()
	if err := driver.EnsureSchema(ctx); err != nil {
		return nil, fmt.Errorf("向量检索: 初始化向量表失败: %w", err)
	}

	uintKBIDs := make([]uint, 0, len(params.KnowledgeBaseIDs))
	for _, id := range params.KnowledgeBaseIDs {
		uintKBIDs = append(uintKBIDs, parseUint(id))
	}
	topK := params.EmbeddingTopK
	if topK <= 0 {
		topK = 5
	}
	results, err := driver.Search(ctx, PipelineVectorSearchParams{
		UserID:           params.UserID,
		VectorStoreID:    store.ID,
		KnowledgeBaseIDs: uintKBIDs,
		QueryVector:      queryVector,
		TopK:             topK,
		Threshold:        params.VectorThreshold,
	})
	if err != nil {
		return nil, fmt.Errorf("向量检索: 检索失败: %w", err)
	}

	// 5. 转换结果
	searchResults := make([]SearchResult, 0, len(results))
	for _, r := range results {
		searchResults = append(searchResults, SearchResult{
			ChunkID:        r.ChunkID,
			Score:          r.Score,
			KnowledgeID:    r.KnowledgeID,
			Content:        r.Content,
			KnowledgeTitle: r.KnowledgeTitle,
		})
	}

	// 6. Rerank 重排（未配置模型或失败时使用原始结果）
	return rerankSearchResults(ctx, deps, params, searchResults), nil
}

// rerankSearchResults 对检索结果重排（与 node_rerank 逻辑一致）
func rerankSearchResults(ctx context.Context, deps *PipelineDeps, params *SearchKBParams, searchResults []SearchResult) []SearchResult {
	if params.RerankModelID == "" || deps.RerankerFactory == nil || len(searchResults) == 0 {
		return searchResults
	}
	reranker, err := deps.RerankerFactory.CreateReranker(ctx, params.RerankModelID)
	if err != nil {
		return searchResults
	}
	documents := make([]string, len(searchResults))
	for i, r := range searchResults {
		documents[i] = r.Content
	}
	topK := params.RerankTopK
	if topK <= 0 {
		topK = len(searchResults)
	}
	rerankResults, err := reranker.Rerank(ctx, params.Query, documents, topK)
	if err != nil {
		return searchResults
	}
	threshold := params.RerankThreshold
	reranked := make([]SearchResult, 0, len(rerankResults))
	for _, rr := range rerankResults {
		if threshold > 0 && rr.RelevanceScore < threshold {
			continue
		}
		if rr.Index < 0 || rr.Index >= len(searchResults) {
			continue
		}
		original := searchResults[rr.Index]
		original.Score = rr.RelevanceScore
		reranked = append(reranked, original)
	}
	sort.Slice(reranked, func(i, j int) bool {
		return reranked[i].Score > reranked[j].Score
	})
	if len(reranked) > topK {
		reranked = reranked[:topK]
	}
	return reranked
}
