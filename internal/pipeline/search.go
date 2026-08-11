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
	KeywordTopK      int
	KeywordThreshold float64
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

	// 1. 过滤无效知识库（已删除/不存在），全部无效时返回空结果而非报错：
	// 配置残留的失效 ID 不应让整次检索失败（否则工具侧包装降级文案会诱导模型重试，耗尽迭代上限）
	validIDs := make([]string, 0, len(params.KnowledgeBaseIDs))
	for _, idStr := range params.KnowledgeBaseIDs {
		kb, err := deps.KBRepo.FindByID(parseUint(idStr))
		if err != nil {
			return nil, fmt.Errorf("向量检索: 查询知识库失败: %s", idStr)
		}
		if kb == nil {
			continue // 知识库已删除或不存在，跳过
		}
		validIDs = append(validIDs, idStr)
	}
	if len(validIDs) == 0 {
		return []SearchResult{}, nil
	}

	// 2. 根据第一个有效知识库确定 Embedding 模型
	kbID := parseUint(validIDs[0])
	kb, _ := deps.KBRepo.FindByID(kbID)

	embedderID := kb.EmbeddingModelID
	if embedderID == "" {
		embedderID = "default"
	}

	// 3. 创建 Embedder 并生成查询向量
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

	uintKBIDs := make([]uint, 0, len(validIDs))
	for _, id := range validIDs {
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

	// 6. 关键字检索（BM25，与向量检索并行一路；失败或未注入时降级为空结果）
	keywordResults := []SearchResult{}
	if deps.KeywordSearch != nil {
		kTopK := params.KeywordTopK
		if kTopK <= 0 {
			kTopK = 5
		}
		keywordResults, err = deps.KeywordSearch.Search(ctx, PipelineKeywordSearchParams{
			KnowledgeBaseIDs: uintKBIDs,
			Query:            params.Query,
			TopK:             kTopK,
			Threshold:        params.KeywordThreshold,
		})
		if err != nil {
			fmt.Printf("[SearchKB] KeywordSearch: 检索失败，降级为空结果: %v\n", err)
			keywordResults = []SearchResult{}
		}
		fmt.Printf("[SearchKB] KeywordSearch: query=%s, kbIDs=%v, results=%d\n",
			params.Query, params.KnowledgeBaseIDs, len(keywordResults))
	}

	// 7. RRF 融合（只比排名不比分数，融合结果写回后统一进 rerank）
	effTopK := params.RerankTopK
	if effTopK <= 0 {
		effTopK = 5
	}
	searchResults = fuseRRFResults(searchResults, keywordResults, effTopK)
	fmt.Printf("[SearchKB] RRFFusion: vector=%d + keyword=%d → fused=%d (topK=%d)\n",
		len(results), len(keywordResults), len(searchResults), effTopK)

	// 8. Rerank 重排（未配置模型或失败时使用融合结果）
	return rerankSearchResults(ctx, deps, params, searchResults), nil
}

// fuseRRFResults 将向量与关键字两路结果按倒数排名融合（与 node_rrf_fusion 算法一致）
// RRF(d) = Σ 1/(k + rank_i(d))，只比排名不比分数（两路分数量纲不同），天然去重
func fuseRRFResults(vectorResults, keywordResults []SearchResult, topK int) []SearchResult {
	if len(vectorResults) == 0 && len(keywordResults) == 0 {
		return []SearchResult{}
	}
	if len(vectorResults) == 0 {
		return keywordResults
	}
	if len(keywordResults) == 0 {
		return vectorResults
	}

	// 1. 按排名累加 RRF 分数（同一 chunk 双路命中自动合并贡献）
	fused := make(map[uint]float64, len(vectorResults)+len(keywordResults))
	for rank, r := range vectorResults {
		fused[r.ChunkID] += 1.0 / (rrfK + float64(rank+1))
	}
	for rank, r := range keywordResults {
		fused[r.ChunkID] += 1.0 / (rrfK + float64(rank+1))
	}

	// 2. 还原条目信息（双路命中时优先取向量路条目，内容一致）
	entries := make(map[uint]SearchResult, len(vectorResults)+len(keywordResults))
	for _, r := range vectorResults {
		entries[r.ChunkID] = r
	}
	for _, r := range keywordResults {
		if _, ok := entries[r.ChunkID]; !ok {
			entries[r.ChunkID] = r
		}
	}

	// 3. 按 RRF 分数降序排序并截断到 topK
	chunkIDs := make([]uint, 0, len(fused))
	for id := range fused {
		chunkIDs = append(chunkIDs, id)
	}
	sort.Slice(chunkIDs, func(i, j int) bool {
		return fused[chunkIDs[i]] > fused[chunkIDs[j]]
	})
	if topK <= 0 {
		topK = 5
	}
	if len(chunkIDs) > topK {
		chunkIDs = chunkIDs[:topK]
	}

	// 4. Score 替换为 RRF 分数（下游 rerank 会重新打分）
	results := make([]SearchResult, 0, len(chunkIDs))
	for _, id := range chunkIDs {
		item := entries[id]
		item.Score = fused[id]
		results = append(results, item)
	}
	return results
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
