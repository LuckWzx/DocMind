package memory

import (
	"github.com/cloudwego/eino/schema"
)

// 轮数触发分档：按模型上下文窗口大小决定"增量轮数压缩阈值"。
//
// 背景：纯 token 阈值触发时，低 token 密度对话（每轮都很短）可能长期不触发压缩，
// 历史无限膨胀；而固定轮数触发又不适配不同上下文大小的模型。
// 分档策略：上下文越大阈值越高——1M 模型轻松吃下数百轮，少压缩少摘要损耗。
// 返回 0 表示未知窗口（退化为纯 token 触发）。
func TurnsThresholdForWindow(contextWindow int) int {
	switch {
	case contextWindow <= 0:
		return 0
	case contextWindow < 32*1024:
		return 10
	case contextWindow < 128*1024:
		return 30
	case contextWindow < 512*1024:
		return 60
	default:
		return 100
	}
}

// CountUserTurns 统计消息列表中的对话轮数（一条 user 消息 = 一轮）。
// 会话存储只含 user / assistant / system 消息，一轮以 user 消息为锚点计数，
// 与 tailKeepStart（从尾部数 user 消息）的轮次口径一致。
func CountUserTurns(messages []*schema.Message) int {
	turns := 0
	for _, m := range messages {
		if m != nil && m.Role == schema.User {
			turns++
		}
	}
	return turns
}
