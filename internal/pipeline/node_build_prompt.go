package pipeline

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/cloudwego/eino/schema"
)

const defaultSystemPrompt = `你是一个知识库问答助手。请根据以下检索到的文档内容回答用户的问题。

规则：
1. 只根据提供的文档内容回答，不要编造信息
2. 如果文档中没有相关信息，请明确告知用户
3. 回答要简洁、准确
4. 在回答中适当引用文档来源`

// imageCitationRule 检索结果包含图片时追加的引用规则
const imageCitationRule = `

[图片引用规则] 若参考文档中包含图片（形如 ![描述](地址) 的 Markdown 图片链接），当回答内容与图片相关时，请把对应的图片链接以 Markdown 图片语法原样输出到回答中的合适位置。不要删除、改写或替换图片地址，不要只输出图片链接而不提供相关文字说明。`

// markdownImageLinkPattern 匹配 Markdown 图片链接
var markdownImageLinkPattern = regexp.MustCompile(`!\[[^\]]*\]\([^)]+\)`)

// buildPromptNode Prompt 拼接节点
func buildPromptNode(ctx context.Context, input *Context) (*Context, error) {
	// 1. 构建 System Prompt
	systemPrompt := input.AgentConfig.SystemPrompt
	if systemPrompt == "" {
		systemPrompt = defaultSystemPrompt
	}

	// 2. 拼接检索到的文档
	if len(input.RerankedResults) > 0 {
		contextText := buildContextText(input.RerankedResults)
		systemPrompt += "\n\n参考文档：\n" + contextText

		// 3. 检索结果包含图片时，指示模型在回答中引用图片
		if contextContainsImages(input.RerankedResults) {
			systemPrompt += imageCitationRule
		}
	} else {
		// 没有检索到内容时，明确告知模型
		systemPrompt += "\n\n[注意] 本次检索未找到任何相关文档内容。请直接告知用户未找到相关信息，不要编造内容。"
	}

	// 3. 构建消息列表：system + 历史消息 + 当前 query
	messages := []*schema.Message{
		{Role: schema.System, Content: systemPrompt},
	}
	messages = append(messages, input.HistoryMessages...)
	messages = append(messages, &schema.Message{
		Role:    schema.User,
		Content: input.Query,
	})
	input.Messages = messages

	fmt.Printf("[Pipeline] BuildPrompt: %d results, %d history, system prompt length=%d\n",
		len(input.RerankedResults), len(input.HistoryMessages), len(systemPrompt))

	return input, nil
}

// buildContextText 构建上下文文本
func buildContextText(results []SearchResult) string {
	var sb strings.Builder
	for i, r := range results {
		sb.WriteString(fmt.Sprintf("[文档 %d] %s\n%s\n\n", i+1, r.KnowledgeTitle, r.Content))
	}
	return sb.String()
}

// contextContainsImages 检测检索结果中是否包含 Markdown 图片链接
func contextContainsImages(results []SearchResult) bool {
	for _, r := range results {
		if markdownImageLinkPattern.MatchString(r.Content) {
			return true
		}
	}
	return false
}
