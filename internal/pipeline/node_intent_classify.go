package pipeline

import (
	"context"
	"fmt"
	"strings"

	einoschema "github.com/cloudwego/eino/schema"
)

const intentClassifyPrompt = `你是一个意图分类器。请判断用户输入属于以下哪个类别，只输出类别名称，不要输出其他内容。

类别说明：
- greeting: 用户打招呼、问候，如"你好"、"早上好"、"hello"等
- chitchat: 与知识库无关的闲聊、询问助手身份能力等，如"你是谁"、"今天天气怎么样"、"给我讲个笑话"等
- kb_search: 需要查询知识库才能回答的问题，如具体业务问题、文档相关问题、产品功能问题等

用户输入：`

// intentClassifyNode 意图分类节点
// 使用 LLM 判断用户意图：kb_search / greeting / chitchat
func intentClassifyNode(ctx context.Context, input *Context) (*Context, error) {
	query := strings.TrimSpace(input.RewrittenQuery)
	if query == "" {
		input.Intent = "kb_search"
		return input, nil
	}

	// 尝试使用 LLM 进行意图分类
	intent, err := llmClassifyIntent(ctx, input)
	if err != nil {
		fmt.Printf("[Pipeline] IntentClassify: LLM 分类失败，回退到关键词匹配: %v\n", err)
		intent = classifyIntentByKeywords(query)
	}

	input.Intent = intent
	fmt.Printf("[Pipeline] IntentClassify: query=%s, intent=%s\n", query, intent)

	// 如果是问候或闲聊，跳过检索
	if intent == "greeting" || intent == "chitchat" {
		input.SearchResults = nil
	}

	return input, nil
}

// llmClassifyIntent 使用 LLM 进行意图分类
func llmClassifyIntent(ctx context.Context, input *Context) (string, error) {
	if input.ModelRepo == nil {
		return "", fmt.Errorf("ModelRepo 未注入")
	}

	// 创建 ChatModel
	chatModel, err := createChatModel(ctx, input.ModelRepo, input.AgentConfig.ModelID, input.UserID)
	if err != nil {
		return "", fmt.Errorf("创建 ChatModel 失败: %w", err)
	}

	// 构造分类请求：system + user
	classifyMsgs := []*einoschema.Message{
		{Role: einoschema.System, Content: "你是一个意图分类器，只输出分类标签。"},
		{Role: einoschema.User, Content: intentClassifyPrompt + input.RewrittenQuery},
	}

	// 使用 Generate 非流式调用
	resp, err := chatModel.Generate(ctx, classifyMsgs)
	if err != nil {
		return "", fmt.Errorf("LLM 调用失败: %w", err)
	}

	raw := strings.TrimSpace(resp.Content)
	raw = strings.ToLower(raw)

	// 解析 LLM 输出，匹配合法意图标签
	validIntents := map[string]bool{
		"greeting":  true,
		"chitchat":  true,
		"kb_search": true,
	}
	// 尝试精确匹配
	if validIntents[raw] {
		return raw, nil
	}
	// 尝试提取：LLM 可能会输出 "意图是 kb_search" 之类的内容
	for intent := range validIntents {
		if strings.Contains(raw, intent) {
			return intent, nil
		}
	}

	return "", fmt.Errorf("无法解析 LLM 返回的意图: %q", raw)
}

// classifyIntentByKeywords 关键词匹配作为 fallback
func classifyIntentByKeywords(query string) string {
	query = strings.ToLower(query)

	// 问候
	greetings := []string{"你好", "您好", "hi", "hello", "嗨", "早上好", "下午好", "晚上好"}
	for _, g := range greetings {
		if strings.Contains(query, g) {
			return "greeting"
		}
	}

	// 闲聊
	chitchat := []string{"你是谁", "你叫什么", "你能做什么", "介绍一下你自己"}
	for _, c := range chitchat {
		if strings.Contains(query, c) {
			return "chitchat"
		}
	}

	// 默认：知识库搜索
	return "kb_search"
}
