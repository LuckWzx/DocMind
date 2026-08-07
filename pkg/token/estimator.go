package token

// Package token 提供 LLM 上下文窗口的 Token 估算能力。
//
// 权威 Token 计数来自模型 API 返回的 Usage 字段（schema.ResponseMeta.Usage），
// 本估算器用于两种场景：
//
//  1. 增量估算：LLM 调用后新产生的消息（assistant 回复 + 工具结果）在下一轮调用前
//     需要估算增量成本，用于判断是否触发上下文压缩，避免额外一次 API 往返。
//  2. 首轮兜底：会话第一轮尚无 API Usage 可用，估算器提供全量估算。
//
// 编码采用 cl100k_base（OpenAI BPE），对不同模型家族是近似值，但对"是否超过
// 触发阈值"的判断足够准确——少量偏差会在下一次 API 调用后被 Usage 纠正。

import (
	"fmt"

	"github.com/cloudwego/eino/schema"
	"github.com/tiktoken-go/tokenizer"
)

const (
	// perMessageOverhead 每条消息的固定 Token 开销（OpenAI 消息计价规则）
	perMessageOverhead = 3
	// perConversationTail 整段消息列表的收尾开销
	perConversationTail = 3
	// perToolCallOverhead 每个工具调用的固定 Token 开销
	perToolCallOverhead = 4
)

// Estimator 使用 BPE 分词估算消息与字符串的 Token 数
type Estimator struct {
	codec tokenizer.Codec
}

// NewEstimator 创建使用 cl100k_base 编码的估算器
func NewEstimator() (*Estimator, error) {
	codec, err := tokenizer.Get(tokenizer.Cl100kBase)
	if err != nil {
		return nil, fmt.Errorf("token: 初始化 tokenizer 失败: %w", err)
	}
	return &Estimator{codec: codec}, nil
}

// EstimateMessages 估算多条消息的总 Token 数
func (e *Estimator) EstimateMessages(messages []*schema.Message) int {
	total := 0
	for _, msg := range messages {
		total += e.EstimateMessage(msg)
	}
	total += perConversationTail
	return total
}

// EstimateMessage 估算单条消息的 Token 数：
// 固定开销 + role + content + name + 每个 tool_call（name + arguments + 固定开销）
func (e *Estimator) EstimateMessage(msg *schema.Message) int {
	if msg == nil {
		return 0
	}
	tokens := perMessageOverhead
	tokens += e.EstimateString(string(msg.Role))
	tokens += e.EstimateString(msg.Name)
	tokens += e.EstimateString(msg.Content)
	tokens += e.EstimateString(msg.ReasoningContent)

	// 多模态输出（assistant 生成的多段内容）
	for _, part := range msg.AssistantGenMultiContent {
		if part.Text != "" {
			tokens += e.EstimateString(part.Text)
		}
	}

	for _, tc := range msg.ToolCalls {
		tokens += e.EstimateString(tc.Function.Name)
		tokens += e.EstimateString(tc.Function.Arguments)
		tokens += perToolCallOverhead
	}

	return tokens
}

// EstimateString 估算纯字符串的 Token 数；
// 分词失败时退化为 4 字符/token 的粗略估算，保证不 panic
func (e *Estimator) EstimateString(s string) int {
	if len(s) == 0 {
		return 0
	}
	ids, _, err := e.codec.Encode(s)
	if err != nil {
		return (len(s) + 3) / 4
	}
	return len(ids)
}
