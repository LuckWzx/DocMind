package chat

// ===== SSE 事件类型常量池 =====
// sseEvent.ResponseType 字段的取值，前端据此区分事件语义

const (
	// SSEEventError 错误事件，Content / ErrorMessage 携带错误信息
	SSEEventError = "error"
	// SSEEventReferences 检索引用事件，References 携带命中的知识分块
	SSEEventReferences = "references"
	// SSEEventAnswer 流式回答事件，Content 携带增量文本
	SSEEventAnswer = "answer"
	// SSEEventSessionTitle 会话标题事件，Content 携带自动生成的会话标题
	SSEEventSessionTitle = "session_title"
	// SSEEventComplete 流结束事件，Done 为 true
	SSEEventComplete = "complete"
	// SSEEventState 状态机事件（thinking/searching/generating/cancelled），State 携带状态值
	SSEEventState = "state"
	// SSEEventThink 模型思考内容事件，Content 携带推理文本
	SSEEventThink = "think"
	// SSEEventAgentStep 工具步骤事件（agent_steps 逐步推送），ToolName / ToolArgs / ToolResult 携带步骤信息
	SSEEventAgentStep = "agent_step"
)

// sseProtocolEvent SSE 协议层事件名，所有业务事件统一用 message 承载，由 response_type 区分
const sseProtocolEvent = "message"
