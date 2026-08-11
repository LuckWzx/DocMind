package pipeline

import (
	"context"
	"fmt"
)

// newKeywordSearchNode 创建 BM25 关键词检索节点（闭包注入外部依赖）
// 与向量检索并行，结果写入 Context.KeywordResults，供后续 RRF 融合使用
func newKeywordSearchNode(deps *PipelineDeps) func(ctx context.Context, input *Context) (*Context, error) {
	return func(ctx context.Context, input *Context) (*Context, error) {
		// 没有知识库或依赖未注入时，降级为空结果
		if len(input.AgentConfig.KnowledgeBaseIDs) == 0 {
			input.KeywordResults = []SearchResult{}
			return input, nil
		}
		if deps == nil || deps.KeywordSearch == nil || deps.DisableBM25 {
			input.KeywordResults = []SearchResult{}
			return input, nil
		}

		// 1. 查询改写后的文本作为检索 query
		query := input.RewrittenQuery
		if query == "" {
			query = input.Query
		}

		// 2. 转换知识库 ID（已按用户隔离）
		uintKBIDs := make([]uint, 0, len(input.AgentConfig.KnowledgeBaseIDs))
		for _, id := range input.AgentConfig.KnowledgeBaseIDs {
			uintKBIDs = append(uintKBIDs, parseUint(id))
		}

		// 3. 确定 TopK（KeywordTopK<=0 时按原默认 5，与关闭开关无关）
		topK := input.AgentConfig.KeywordTopK
		if topK <= 0 {
			topK = 5
		}

		// 4. 执行 BM25 检索
		results, err := deps.KeywordSearch.Search(ctx, PipelineKeywordSearchParams{
			KnowledgeBaseIDs: uintKBIDs,
			Query:            query,
			TopK:             topK,
			Threshold:        input.AgentConfig.KeywordThreshold,
		})
		if err != nil {
			// 检索失败不阻塞主链路，降级为空结果
			fmt.Printf("[Pipeline] KeywordSearch: 检索失败，降级为空结果: %v\n", err)
			input.KeywordResults = []SearchResult{}
			return input, nil
		}

		input.KeywordResults = results
		fmt.Printf("[Pipeline] KeywordSearch: query=%s, kbIDs=%v, results=%d\n",
			query, input.AgentConfig.KnowledgeBaseIDs, len(results))

		return input, nil
	}
}
