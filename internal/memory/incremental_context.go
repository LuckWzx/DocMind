package memory

import (
	"context"
	"fmt"
	"time"

	"docmind/internal/model/entity"
	"docmind/pkg/logger"

	"github.com/cloudwego/eino/schema"
)

// buildAgentContextMaxIncremental 增量加载上限（与 chat_service 保持一致）
const buildAgentContextMaxIncremental = 500

// SummaryStore 摘要存储接口：由 repository.SummaryRepository 满足。
// 定义在本包而非 repository，避免 memory 反向依赖 repository 包。
type SummaryStore interface {
	GetBySession(sessionID uint) (*entity.SessionSummary, error)
	Upsert(summary *entity.SessionSummary) error
}

// MessageLoader 消息加载接口：由 repository.MessageRepository 满足。
type MessageLoader interface {
	ListAfterID(sessionID uint, afterID uint, limit int) ([]*entity.Message, error)
	ListBySession(sessionID uint, limit int, beforeTime *time.Time) ([]*entity.Message, error)
}

// BuildAgentContext 组装 Agent 模式的短期记忆上下文（跨请求增量压缩的调用方入口）。
//
// 背景：官方 summarization 中间件只在单次 Agent 运行内生效（state 常驻），
// 跨请求时每次运行都从数据库重建消息列表，摘要不持久化 → 每次全量重算。
// 本函数把"读摘要 → 增量加载 → 触发压缩 → 写回 → 拼装"封装为一次调用，
// 供 Agent 引擎调用方在组装 RunRequest.Messages 时使用；官方中间件保留
// 作为单次运行内（长工具链）的兜底。
//
// 用法（二选一）：
//   - 先 messageRepo.Create 保存当前问题，再调用（currentUserMsg 传 nil，
//     函数把加载结果整体当增量，最后一条 user 即当前轮，与 quick-answer 一致）；
//   - 不保存当前问题，直接传 currentUserMsg（追加为增量最后一条）。
//
// 返回完整消息列表：[摘要 system 消息（若有）] + 增量保留部分 + 当前轮；
// 触发压缩时摘要已写回（LastMessageID 边界），压缩失败降级为原文归档不阻断。
//
// 注意：historyTurns 参数已废弃（历史加载量由短期记忆压缩机制接管，
// 无摘要时全量加载），仅保留用于兼容旧调用方，传任意值均不影响行为。
func BuildAgentContext(
	ctx context.Context,
	sessionID uint,
	summaryStore SummaryStore,
	messageLoader MessageLoader,
	consolidator *Consolidator,
	historyTurns int,
	currentUserMsg *schema.Message,
) ([]*schema.Message, error) {
	if summaryStore == nil || messageLoader == nil || consolidator == nil {
		return nil, fmt.Errorf("BuildAgentContext: summaryStore / messageLoader / consolidator 不能为空")
	}

	// 1. 读旧摘要
	summary, err := summaryStore.GetBySession(sessionID)
	if err != nil {
		return nil, fmt.Errorf("读取会话摘要失败: %w", err)
	}

	// 2. 加载增量消息（保留 DB ID，写回压缩边界用）
	var loaded []*entity.Message
	if summary != nil {
		loaded, err = messageLoader.ListAfterID(sessionID, summary.LastMessageID, buildAgentContextMaxIncremental)
	} else {
		// 首次：全量加载历史（不再受轮数限制），让 Token 自然累积触发首份摘要
		loaded, err = messageLoader.ListBySession(sessionID, buildAgentContextMaxIncremental, nil)
	}
	if err != nil {
		return nil, fmt.Errorf("加载增量消息失败: %w", err)
	}

	// 3. 组装增量列表（含当前轮）
	boundaryIDs := make([]uint, 0, len(loaded)+1)
	incremental := make([]*schema.Message, 0, len(loaded)+1)
	for _, m := range loaded {
		boundaryIDs = append(boundaryIDs, m.ID)
		incremental = append(incremental, toSchemaMsg(m))
	}
	if currentUserMsg != nil {
		incremental = append(incremental, currentUserMsg)
	}

	// 4. 拼装：摘要（若有）+ 增量
	summaryContent := ""
	if summary != nil {
		summaryContent = summary.Content
	}
	result := make([]*schema.Message, 0, len(incremental)+1)
	if summaryContent != "" {
		result = append(result, &schema.Message{Role: schema.System, Content: summaryContent})
	}
	result = append(result, incremental...)

	// 5. 触发增量压缩 → 写回 → 本次请求使用新摘要 + 保留的增量
	if len(incremental) > 1 {
		currentTokens := consolidator.estimator.EstimateString(summaryContent) +
			consolidator.estimator.EstimateMessages(incremental)
		if consolidator.ShouldConsolidate(currentTokens) {
			newSummary, count, isRaw := consolidator.ConsolidateIncremental(ctx, summaryContent, incremental)
			if count > 0 {
				summaryType := entity.SummaryTypeLLM
				if isRaw {
					summaryType = entity.SummaryTypeRaw
				}
				upsert := &entity.SessionSummary{
					SessionID:       sessionID,
					Content:         newSummary,
					SummaryType:     summaryType,
					LastMessageID:   boundaryIDs[count-1],
					CompressedCount: count,
				}
				if summary != nil {
					upsert.CompressedCount += summary.CompressedCount
				}
				if err := summaryStore.Upsert(upsert); err != nil {
					logger.Warnf("[MemoryConsolidator] Agent 摘要写回失败（不影响本次运行）: %v", err)
				}
				result = append([]*schema.Message{{
					Role:    schema.System,
					Content: newSummary,
				}}, incremental[count:]...)
			}
		}
	}
	return result, nil
}

// toSchemaMsg 将数据库消息转换为 eino schema 消息（Agent 增量上下文组装用）
func toSchemaMsg(m *entity.Message) *schema.Message {
	role := schema.User
	if m.Role == "assistant" {
		role = schema.Assistant
	} else if m.Role == "system" {
		role = schema.System
	}
	return &schema.Message{
		Role:    role,
		Content: m.Content,
	}
}
