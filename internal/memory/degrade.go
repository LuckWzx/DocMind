package memory

import (
	"context"
	"time"

	"docmind/pkg/logger"

	"github.com/cloudwego/eino/adk"
)

// degradableMiddleware 降级包装层。
//
// 官方 summarization 中间件在摘要生成彻底失败（重试 + 降级模型均失败）时，
// BeforeModelRewriteState 直接返回错误，会导致整轮 Agent 中断。
// 本层捕获该错误并降级为"原文归档压缩"，保证对话不中断——
// 宁可牺牲摘要质量（降级为原始文本截断），也不阻断主流程。
type degradableMiddleware struct {
	*adk.BaseChatModelAgentMiddleware // 嵌入 no-op 实现，只覆盖需要的方法
	inner                             adk.ChatModelAgentMiddleware
	preserveTurns                     int
	timeout                           time.Duration
}

// BeforeModelRewriteState 每次模型调用前的钩子：转发给官方中间件，失败时降级
func (m *degradableMiddleware) BeforeModelRewriteState(ctx context.Context, state *adk.ChatModelAgentState, mc *adk.ModelContext) (context.Context, *adk.ChatModelAgentState, error) {
	// 摘要生成使用独立超时上下文；超时只约束摘要，不影响后续主模型调用
	summaryCtx := ctx
	cancel := func() {}
	if m.timeout > 0 {
		summaryCtx, cancel = context.WithTimeout(ctx, m.timeout)
	}
	defer cancel()

	_, newState, err := m.inner.BeforeModelRewriteState(summaryCtx, state, mc)
	if err == nil {
		return ctx, newState, nil
	}

	logger.Warnf("[MemorySummary] 摘要生成失败，降级为原文归档: %v", err)

	degraded := *state
	degraded.Messages = degradeMessages(state.Messages, m.preserveTurns)
	return ctx, &degraded, nil
}
