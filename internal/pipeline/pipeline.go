package pipeline

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/compose"
)

// Pipeline RAG 流水线
type Pipeline struct {
	runnable compose.Runnable[*Context, *Context]
	deps     *PipelineDeps
}

// NewPipeline 创建 RAG 流水线
func NewPipeline(deps *PipelineDeps) (*Pipeline, error) {
	g := compose.NewGraph[*Context, *Context]()

	// 添加节点（使用 InvokableLambda）
	queryRewriteLambda := compose.InvokableLambda(queryRewriteNode)
	intentClassifyLambda := compose.InvokableLambda(intentClassifyNode)
	vectorSearchLambda := compose.InvokableLambda(newVectorSearchNode(deps))
	rerankLambda := compose.InvokableLambda(newRerankNode(deps))
	buildPromptLambda := compose.InvokableLambda(buildPromptNode)
	chatCompletionLambda := compose.InvokableLambda(chatCompletionNode)

	_ = g.AddLambdaNode("query_rewrite", queryRewriteLambda)
	_ = g.AddLambdaNode("intent_classify", intentClassifyLambda)
	_ = g.AddLambdaNode("vector_search", vectorSearchLambda)
	_ = g.AddLambdaNode("rerank", rerankLambda)
	_ = g.AddLambdaNode("build_prompt", buildPromptLambda)
	_ = g.AddLambdaNode("chat_completion", chatCompletionLambda)

	// 连接节点
	_ = g.AddEdge(compose.START, "query_rewrite")
	_ = g.AddEdge("query_rewrite", "intent_classify")
	_ = g.AddEdge("intent_classify", "vector_search")
	_ = g.AddEdge("vector_search", "rerank")
	_ = g.AddEdge("rerank", "build_prompt")
	_ = g.AddEdge("build_prompt", "chat_completion")
	_ = g.AddEdge("chat_completion", compose.END)

	// 编译图
	runnable, err := g.Compile(context.Background())
	if err != nil {
		return nil, fmt.Errorf("编译 pipeline 失败: %w", err)
	}

	return &Pipeline{runnable: runnable, deps: deps}, nil
}

// Run 执行流水线
func (p *Pipeline) Run(ctx context.Context, input *Context) (*Context, error) {
	// 将外部依赖注入到 Context 中，供各节点使用
	input.PipelineDeps = p.deps

	result, err := p.runnable.Invoke(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("pipeline 执行失败: %w", err)
	}
	return result, nil
}
