package memory

import (
	"context"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// 短期记忆中间件默认值
const (
	// DefaultMaxContextTokens 模型上下文窗口默认 Token 数（与官方中间件默认触发阈值一致）
	DefaultMaxContextTokens = 160000
	// DefaultThresholdRatio 触发阈值比例：上下文窗口 × 50%
	DefaultThresholdRatio = 0.5
	// DefaultConsolidationThreshold quick-answer Consolidator 的触发比例（与 Agent 模式一致）
	DefaultConsolidationThreshold = 0.5
	// DefaultPreserveTurns 压缩后保留的最近完整轮数（tool_call 配对保护）
	DefaultPreserveTurns = 5
	// DefaultSummaryTimeout 单次摘要生成超时
	DefaultSummaryTimeout = 60 * time.Second
	// DefaultSummaryRetries 摘要生成失败重试次数（官方默认 3）
	DefaultSummaryRetries = 3
	// DefaultFailoverRetries 摘要生成失败后降级模型尝试次数（官方默认 3）
	DefaultFailoverRetries = 3
)

// SummaryOptions 短期记忆中间件配置（Agent 模式）
//
// 对外只暴露业务关心的配置项，官方中间件的内部细节（Retry/Failover/事件等）
// 由 NewSummaryMiddleware 在工厂内消化，避免集成方（乙）配置出错。
type SummaryOptions struct {
	// CreateModel 创建摘要模型的工厂函数（必填）
	// 由调用方提供，示例：func(ctx) { return chatModelFactory.CreateChatModel(ctx, agent.ModelID) }
	CreateModel func(ctx context.Context) (model.BaseModel[*schema.Message], error)

	// MaxContextTokens 模型上下文窗口 Token 数（默认 160000）
	// 触发阈值 = MaxContextTokens × 50%
	MaxContextTokens int

	// ContextMessages 按消息数触发（>0 生效，与 Token 阈值任一满足即触发）
	ContextMessages int

	// PreserveTurns 压缩后保留的最近完整轮数（默认 5）
	// 保留部分从最近的 user 消息起整体截取，同一轮内的 assistant tool_call 与
	// tool result 天然配对完整，杜绝半截配对。
	PreserveTurns int

	// UserInstruction 覆盖官方默认摘要指令（可选，默认使用官方内置的中英双语模板）
	UserInstruction string

	// TranscriptFilePath 完整对话记录文件路径（可选）
	// 设置后摘要中会附带"如需细节可读原文件"的提示，压缩前的历史可回溯。
	TranscriptFilePath string

	// Timeout 单次摘要生成超时（默认 60s）
	Timeout time.Duration

	// Retries 摘要生成失败重试次数（默认 3）
	Retries int

	// FailoverRetries 摘要失败后降级模型尝试次数（默认 3）
	FailoverRetries int

	// EmitEvents 是否将压缩过程（before/after/generate）作为 Agent 事件推入事件流
	// （默认 false）。仅在完整 Agent 引擎（adk.Runner）内运行时开启，可用于前端
	// 提示"历史已压缩"；独立调用中间件（单测 / 直接调 BeforeModelRewriteState）
	// 时保持关闭，否则官方中间件会因缺少事件通道报错。
	EmitEvents bool
}
