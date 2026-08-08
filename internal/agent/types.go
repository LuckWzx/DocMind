package agent

import (
	"context"

	"docmind/internal/model/entity"

	"github.com/cloudwego/eino/schema"
)

// OutputEventType 引擎输出事件类型（与前端 SSE response_type 对齐）
type OutputEventType string

const (
	EventState      OutputEventType = "state"      // 状态机事件（thinking/searching/generating/cancelled）
	EventThink      OutputEventType = "think"      // 模型思考内容（reasoning）
	EventStep       OutputEventType = "agent_step" // 工具步骤（声明/结果，规划 3.2.5）
	EventAnswer     OutputEventType = "answer"     // 回答增量（流式）
	EventReferences OutputEventType = "references" // 引用来源
	EventError      OutputEventType = "error"      // 错误
	EventComplete   OutputEventType = "complete"   // 结束
)

// OutputEvent 引擎输出的统一事件（SSE 层直接映射，规划 3.2.7）
type OutputEvent struct {
	Type       OutputEventType    `json:"response_type"`
	Content    string             `json:"content,omitempty"`
	State      string             `json:"state,omitempty"`
	ToolName   string             `json:"tool_name,omitempty"`
	ToolArgs   string             `json:"tool_args,omitempty"`
	ToolResult string             `json:"tool_result,omitempty"`
	References []entity.Reference `json:"references,omitempty"`
	Done       bool               `json:"done,omitempty"`
}

// HistoryProvider 会话历史加载（多轮对话，Agent 配置 MultiTurnEnabled 时引擎自动加载）
// 由调用方（chat_service 等）注入 repository 实现，引擎保持无数据库依赖
// 返回的 []*schema.Message 需按时间正序（旧 → 新），不含当前用户问题
type HistoryProvider interface {
	LoadHistory(ctx context.Context, sessionID uint, limit int) ([]*schema.Message, error)
}

// RunRequest 一次 Agent 运行的输入
type RunRequest struct {
	SessionID uint
	UserID    uint
	// Messages 历史消息 + 当前用户问题（由调用方组装，优先级最高）；
	// 为空时由引擎自动加载：History（若启用多轮）→ 追加 Question
	Messages []*schema.Message
	// Question 当前用户问题（Messages 为空时使用）
	Question string
	// History 会话历史加载器（可选，多轮对话场景注入）
	History HistoryProvider
	// Agent 智能体实体（含 Config，引擎据此构建运行配置）
	Agent *entity.Agent
}
