package pipeline

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"
)

// 默认改写提示词
const (
	defaultRewriteSystemPrompt = `你是一个查询改写专家。你的任务是将用户的口语化表达改写为更适合向量搜索的查询。

改写规则：
1. 保持原意，但使表达更正式、更准确
2. 消除指代和省略，补全上下文
3. 去除无关的口语化表达
4. 如果用户问题是完整的，可以不改写
5. 只输出改写后的查询，不要输出其他内容`

	defaultRewriteUserPrompt = `请将以下用户查询改写为更适合向量搜索的格式。

用户查询：{{query}}

请输出改写后的查询：`
)

// queryRewriteNode 查询改写节点
// 使用 LLM 将用户口语化表达转换为更适合向量搜索的查询
func queryRewriteNode(ctx context.Context, input *Context) (*Context, error) {
	// 如果没有启用查询改写，直接返回原始查询
	if !input.AgentConfig.EnableQueryRewrite {
		input.RewrittenQuery = input.Query
		return input, nil
	}

	// 调用 LLM 进行查询改写
	rewrittenQuery, err := llmRewriteQuery(ctx, input)
	if err != nil {
		fmt.Printf("[Pipeline] QueryRewrite: LLM 改写失败，使用原始查询: %v\n", err)
		input.RewrittenQuery = input.Query
	} else {
		input.RewrittenQuery = rewrittenQuery
	}

	fmt.Printf("[Pipeline] QueryRewrite: original=%s, rewritten=%s\n", input.Query, input.RewrittenQuery)
	return input, nil
}

// llmRewriteQuery 使用 LLM 进行查询改写
func llmRewriteQuery(ctx context.Context, input *Context) (string, error) {
	if input.ModelRepo == nil {
		return "", fmt.Errorf("ModelRepo 未注入")
	}

	// 确定使用的模型ID：优先使用 QueryUnderstandModelID，否则使用 ModelID
	modelID := input.AgentConfig.QueryUnderstandModelID
	if modelID == "" {
		modelID = input.AgentConfig.ModelID
	}

	// 创建 ChatModel
	chatModel, err := createChatModel(ctx, input.ModelRepo, modelID)
	if err != nil {
		return "", fmt.Errorf("创建 ChatModel 失败: %w", err)
	}

	// 构建系统提示词
	systemPrompt := input.AgentConfig.RewritePromptSystem
	if systemPrompt == "" {
		systemPrompt = defaultRewriteSystemPrompt
	}

	// 构建用户提示词
	userPrompt := input.AgentConfig.RewritePromptUser
	if userPrompt == "" {
		userPrompt = defaultRewriteUserPrompt
	}

	// 替换用户提示词中的占位符
	userPrompt = strings.ReplaceAll(userPrompt, "{{query}}", input.Query)

	// 构造改写请求
	rewriteMsgs := []*schema.Message{
		{Role: schema.System, Content: systemPrompt},
		{Role: schema.User, Content: userPrompt},
	}

	// 使用 Generate 非流式调用
	resp, err := chatModel.Generate(ctx, rewriteMsgs)
	if err != nil {
		return "", fmt.Errorf("LLM 调用失败: %w", err)
	}

	rewritten := strings.TrimSpace(resp.Content)
	if rewritten == "" {
		return input.Query, nil
	}

	return rewritten, nil
}
