package pipeline

import (
	"context"
	"fmt"
	"strings"
	"time"

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
	startTime := time.Now()
	toolCallID := "query_understand"

	fmt.Printf("[Pipeline] QueryRewrite: ===== 开始执行 =====\n")
	fmt.Printf("[Pipeline] QueryRewrite: EnableQueryRewrite=%v\n", input.AgentConfig.EnableQueryRewrite)
	fmt.Printf("[Pipeline] QueryRewrite: Query=%s\n", input.Query)
	fmt.Printf("[Pipeline] QueryRewrite: StepCallback=%v\n", input.StepCallback != nil)

	// 发送步骤开始回调
	if input.StepCallback != nil {
		input.StepCallback(StepInfo{
			StepName:   "query_understand",
			StartTime:  startTime,
			ToolCallID: toolCallID,
			Success:    true,
		})
		fmt.Printf("[Pipeline] QueryRewrite: 已发送步骤开始回调\n")
	}

	// 如果没有启用查询改写，直接返回原始查询
	if !input.AgentConfig.EnableQueryRewrite {
		fmt.Printf("[Pipeline] QueryRewrite: 查询改写未启用，使用原始查询\n")
		input.RewrittenQuery = input.Query
		endTime := time.Now()
		duration := endTime.Sub(startTime).Milliseconds()

		// 发送步骤完成回调
		if input.StepCallback != nil {
			input.StepCallback(StepInfo{
				StepName:   "query_understand",
				StartTime:  startTime,
				EndTime:    endTime,
				Duration:   duration,
				ToolCallID: toolCallID,
				Success:    true,
				Data:       map[string]interface{}{"rewritten_query": input.Query},
			})
		}

		fmt.Printf("[Pipeline] QueryRewrite: disabled, using original query\n")
		return input, nil
	}

	// 调用 LLM 进行查询改写
	rewrittenQuery, err := llmRewriteQuery(ctx, input)
	endTime := time.Now()
	duration := endTime.Sub(startTime).Milliseconds()

	var success bool
	var data map[string]interface{}
	if err != nil {
		fmt.Printf("[Pipeline] QueryRewrite: LLM 改写失败，使用原始查询: %v\n", err)
		input.RewrittenQuery = input.Query
		success = false
		data = map[string]interface{}{"error": err.Error()}
	} else {
		input.RewrittenQuery = rewrittenQuery
		success = true
		data = map[string]interface{}{"rewritten_query": rewrittenQuery}
	}

	// 发送步骤完成回调
	if input.StepCallback != nil {
		input.StepCallback(StepInfo{
			StepName:   "query_understand",
			StartTime:  startTime,
			EndTime:    endTime,
			Duration:   duration,
			ToolCallID: toolCallID,
			Success:    success,
			Data:       data,
		})
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
	chatModel, err := createChatModel(ctx, input.ModelRepo, modelID, input.UserID)
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
