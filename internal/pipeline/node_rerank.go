package pipeline

import (
	"context"
	"fmt"
	"sort"
)

// newRerankNode 创建重排序节点（闭包注入外部依赖）
func newRerankNode(deps *PipelineDeps) func(ctx context.Context, input *Context) (*Context, error) {
	return func(ctx context.Context, input *Context) (*Context, error) {
		// 如果没有配置 Rerank 模型，直接使用原始结果
		if input.AgentConfig.RerankModelID == "" {
			input.RerankedResults = input.SearchResults
			return input, nil
		}

		// 如果没有搜索结果，跳过重排序
		if len(input.SearchResults) == 0 {
			input.RerankedResults = []SearchResult{}
			return input, nil
		}

		// 依赖未注入时降级为原始结果
		if deps == nil || deps.RerankerFactory == nil {
			fmt.Printf("[Pipeline] Rerank: RerankerFactory 未注入，跳过重排序\n")
			input.RerankedResults = input.SearchResults
			return input, nil
		}

		// 1. 创建 Reranker 实例
		reranker, err := deps.RerankerFactory.CreateReranker(ctx, input.AgentConfig.RerankModelID)
		if err != nil {
			fmt.Printf("[Pipeline] Rerank: 创建 Reranker 失败 (%s)，使用原始结果: %v\n",
				input.AgentConfig.RerankModelID, err)
			input.RerankedResults = input.SearchResults
			return input, nil
		}

		// 2. 提取文档内容用于 Rerank
		documents := make([]string, len(input.SearchResults))
		for i, r := range input.SearchResults {
			documents[i] = r.Content
		}

		// 3. 确定 topK
		topK := input.AgentConfig.RerankTopK
		if topK <= 0 {
			topK = len(input.SearchResults)
		}

		// 4. 调用 Rerank API
		rerankResults, err := reranker.Rerank(ctx, input.RewrittenQuery, documents, topK)
		if err != nil {
			fmt.Printf("[Pipeline] Rerank: 调用失败，使用原始结果: %v\n", err)
			input.RerankedResults = input.SearchResults
			return input, nil
		}

		// 5. 根据 Rerank 结果重新排序并过滤低分结果
		threshold := input.AgentConfig.RerankThreshold
		reranked := make([]SearchResult, 0, len(rerankResults))

		// 构建 index → 原始结果的映射
		for _, rr := range rerankResults {
			// 过滤低于阈值的结果
			if threshold > 0 && rr.RelevanceScore < threshold {
				continue
			}
			// 边界检查
			if rr.Index < 0 || rr.Index >= len(input.SearchResults) {
				continue
			}
			original := input.SearchResults[rr.Index]
			original.Score = rr.RelevanceScore // 用 Rerank 分数替换原始向量分数
			reranked = append(reranked, original)
		}

		// 6. 按 Rerank 分数降序排列
		sort.Slice(reranked, func(i, j int) bool {
			return reranked[i].Score > reranked[j].Score
		})

		// 7. 截断到 topK
		if len(reranked) > topK {
			reranked = reranked[:topK]
		}

		input.RerankedResults = reranked
		fmt.Printf("[Pipeline] Rerank: %d → %d results (threshold=%.2f, topK=%d)\n",
			len(input.SearchResults), len(reranked), threshold, topK)

		return input, nil
	}
}
