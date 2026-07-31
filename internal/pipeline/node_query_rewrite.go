package pipeline

import (
	"context"
)

// queryRewriteNode 查询改写节点
// 使用 LLM 将用户口语化表达转换为更适合向量搜索的查询
func queryRewriteNode(ctx context.Context, input *Context) (*Context, error) {
	// 如果没有启用查询改写，直接返回原始查询
	if !input.AgentConfig.EnableQueryRewrite {
		input.RewrittenQuery = input.Query
		return input, nil
	}

	// TODO: 调用 LLM 进行查询改写
	// 这里可以使用 Eino 的 ChatModel 来改写查询
	// 暂时直接使用原始查询
	input.RewrittenQuery = input.Query

	return input, nil
}
