package pipeline

import (
	"context"
	"fmt"
)

// rerankNode 重排序节点
func rerankNode(ctx context.Context, input *Context) (*Context, error) {
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

	// TODO: 调用 Rerank 模型进行重排序
	// 暂时直接使用原始结果
	input.RerankedResults = input.SearchResults

	fmt.Printf("[Pipeline] Rerank: %d results\n", len(input.SearchResults))

	return input, nil
}
