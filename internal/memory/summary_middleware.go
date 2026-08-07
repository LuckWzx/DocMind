package memory

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/middlewares/summarization"
	"github.com/cloudwego/eino/schema"
)

// NewSummaryMiddleware 构建 Agent 模式的短期记忆中间件（交付乙）。
//
// 返回的中间件直接挂进 ChatModelAgentConfig.Handlers，内部组装：
//  1. 官方 summarization 中间件（触发判定 / LLM 摘要 / 重试 / 降级模型）
//  2. 自定义 Finalize：压缩后保留最近 N 轮完整对话（tool_call 配对保护）
//  3. 外层降级包装：摘要生成失败时降级为原文归档，保证对话不中断
func NewSummaryMiddleware(ctx context.Context, opts SummaryOptions) (adk.ChatModelAgentMiddleware, error) {
	if opts.CreateModel == nil {
		return nil, fmt.Errorf("SummaryOptions.CreateModel 不能为空")
	}

	maxContextTokens := opts.MaxContextTokens
	if maxContextTokens <= 0 {
		maxContextTokens = DefaultMaxContextTokens
	}
	preserveTurns := opts.PreserveTurns
	if preserveTurns <= 0 {
		preserveTurns = DefaultPreserveTurns
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = DefaultSummaryTimeout
	}
	retries := opts.Retries
	if retries <= 0 {
		retries = DefaultSummaryRetries
	}
	failoverRetries := opts.FailoverRetries
	if failoverRetries <= 0 {
		failoverRetries = DefaultFailoverRetries
	}

	// 1. 摘要模型（由调用方提供的工厂函数创建，复用 ChatModelFactory）
	summaryModel, err := opts.CreateModel(ctx)
	if err != nil {
		return nil, fmt.Errorf("创建摘要模型失败: %w", err)
	}

	// 2. 官方 summarization 中间件
	inner, err := summarization.New(ctx, &summarization.Config{
		Model: summaryModel,
		Trigger: &summarization.TriggerCondition{
			// 触发阈值 = 上下文窗口 × 50%（阶段二.md 契约）
			ContextTokens:   int(float64(maxContextTokens) * DefaultThresholdRatio),
			ContextMessages: opts.ContextMessages,
		},
		// 摘要生成重试与降级模型（官方默认不重试，需显式开启）
		Retry: &summarization.RetryConfig{
			MaxRetries: intPtr(retries),
		},
		Failover: &summarization.FailoverConfig{
			MaxRetries: intPtr(failoverRetries),
		},
		// 自定义 Finalize：保留最近 N 轮，tool_call 配对完整
		Finalize:           buildFinalize(preserveTurns),
		UserInstruction:    opts.UserInstruction,
		TranscriptFilePath: opts.TranscriptFilePath,
		// 压缩过程以事件形式进入 Agent 事件流，可推送前端"历史已压缩"
		// （仅完整 Agent 引擎内可用，独立调用时保持关闭）
		EmitInternalEvents: opts.EmitEvents,
	})
	if err != nil {
		return nil, fmt.Errorf("创建 summarization 中间件失败: %w", err)
	}

	// 3. 降级包装层
	return &degradableMiddleware{
		inner:         inner,
		preserveTurns: preserveTurns,
		timeout:       timeout,
	}, nil
}

// buildFinalize 自定义 Finalize：
// 压缩结果 = 官方默认后处理（system + 摘要，含用户消息回填与 preamble）
//            + 最近 N 轮完整对话。
//
// 官方默认 Finalize 会把全部历史压成摘要（信息保全依赖摘要质量）；
// 这里保留尾部 N 轮作为双保险——保留边界从最近的 user 消息起整体截取，
// 同一轮内的 assistant tool_call 与 tool result 天然配对，杜绝半截配对。
func buildFinalize(preserveTurns int) summarization.FinalizeFunc {
	return func(ctx context.Context, originalMessages []*schema.Message, summary *schema.Message) ([]*schema.Message, error) {
		if len(originalMessages) <= 2 {
			return originalMessages, nil
		}

		// 1. 官方默认后处理：<all_user_messages> 回填真实用户消息 + preamble
		//    （自定义 Finalize 时官方不再做任何后处理，必须显式调用）
		base, err := summarization.DefaultFinalize(ctx, originalMessages, summary)
		if err != nil {
			return nil, fmt.Errorf("summary 后处理失败: %w", err)
		}

		// 2. 尾部保留 N 轮（tool 配对完整）
		_, contextMsgs := splitSystemMsgs(originalMessages)
		keepStart := tailKeepStart(contextMsgs, preserveTurns)
		if keepStart == 0 {
			// 全部消息不足 N 轮，无需压缩
			return originalMessages, nil
		}

		result := make([]*schema.Message, 0, len(base)+len(contextMsgs)-keepStart)
		result = append(result, base...)
		result = append(result, contextMsgs[keepStart:]...)
		return result, nil
	}
}

// splitSystemMsgs 拆分开头的 system 消息与对话消息
func splitSystemMsgs(msgs []*schema.Message) ([]*schema.Message, []*schema.Message) {
	i := 0
	for i < len(msgs) && msgs[i].Role == schema.System {
		i++
	}
	return msgs[:i], msgs[i:]
}

// tailKeepStart 从消息尾部往前数 turns 个 user 消息，返回保留起点下标；
// 消息不足 turns 轮时返回 0（表示全部保留，不压缩）
func tailKeepStart(messages []*schema.Message, turns int) int {
	userCount := 0
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == schema.User {
			userCount++
			if userCount >= turns {
				return i
			}
		}
	}
	return 0
}

func intPtr(v int) *int {
	return &v
}
