package service

import (
	"context"
	"time"

	"docmind/internal/agent"
	"docmind/internal/agent/tools"
	"docmind/internal/model/entity"
	"docmind/internal/pipeline"

	"github.com/cloudwego/eino/schema"
)

// ChatService 对话服务接口
type ChatService interface {
	// KnowledgeChat 单步 RAG 对话，返回流式响应
	KnowledgeChat(ctx context.Context, sessionID uint, userID uint, req *KnowledgeChatRequest, stepCallback pipeline.StepCallback) (*schema.StreamReader[*schema.Message], []VectorSearchResult, error)
	// AgentChat 智能推理对话（AgentMode=smart-reasoning），返回统一事件流 + 引用收集器（规划 3.2.7 ⑧）
	AgentChat(ctx context.Context, sessionID uint, userID uint, req *AgentChatRequest) (*AgentChatResponse, error)
	// CreateSession 创建会话
	CreateSession(ctx context.Context, userID uint, req *CreateSessionRequest) (*entity.Session, error)
	// GetSession 获取单个会话
	GetSession(ctx context.Context, sessionID uint, userID uint) (*entity.Session, error)
	// ListSessions 获取会话列表
	ListSessions(ctx context.Context, userID uint, source string, page, pageSize int) ([]*entity.Session, int64, error)
	// UpdateSession 更新会话
	UpdateSession(ctx context.Context, sessionID uint, userID uint, req *UpdateSessionRequest) error
	// DeleteSession 删除会话
	DeleteSession(ctx context.Context, sessionID uint, userID uint) error
	// BatchDeleteSessions 批量删除会话
	BatchDeleteSessions(ctx context.Context, userID uint, sessionIDs []uint, deleteAll bool) error
	// PinSession 置顶会话
	PinSession(ctx context.Context, sessionID uint, userID uint, pinned bool) error
	// ClearSessionMessages 清空会话消息
	ClearSessionMessages(ctx context.Context, sessionID uint, userID uint) error
	// LoadMessages 加载历史消息
	LoadMessages(ctx context.Context, sessionID uint, userID uint, limit int, beforeTime *time.Time) ([]*entity.Message, error)
	// SaveAssistantMessage 保存助手消息
	SaveAssistantMessage(ctx context.Context, sessionID uint, content string, references []entity.Reference, agentSteps entity.AgentSteps, isCompleted bool, agentDurationMs int64, isFallback bool) error
	// ResolveChatModelID 解析会话实际使用的对话模型 ID（长期记忆等模块按当前用户会话模型调用）
	ResolveChatModelID(ctx context.Context, userID uint, session *entity.Session) string
	// GenerateSessionTitle 若会话标题仍为默认占位（"新对话"），则用首条用户消息生成标题并落库，返回新标题。已被用户手动重命名的会话不会被覆盖。
	GenerateSessionTitle(ctx context.Context, sessionID uint, userID uint, query string) (string, error)
	// GetMemoryStatus 计算会话短期记忆状态（前端上下文状态条展示；未开启多轮时返回 nil）
	GetMemoryStatus(ctx context.Context, sessionID uint, userID uint) (*MemoryStatus, error)
	// CompressMemory 手动压缩会话短期记忆（增量合并进摘要并写回边界），返回压缩后状态
	CompressMemory(ctx context.Context, sessionID uint, userID uint) (*MemoryStatus, error)
}

// MemoryStatus 会话短期记忆状态（前端上下文状态条数据）
type MemoryStatus struct {
	// ModelID 当前会话对话模型 ID
	ModelID string `json:"model_id"`
	// ContextWindow 模型上下文窗口 Token 数（未知时为 0）
	ContextWindow int `json:"context_window"`
	// CurrentTokens 当前记忆占用 Token（摘要 + 增量 + 当前轮，BPE 估算）
	CurrentTokens int `json:"current_tokens"`
	// TokenThreshold Token 触发阈值（窗口 × 50%）
	TokenThreshold int `json:"token_threshold"`
	// CurrentTurns 距上次压缩的增量轮数（含当前轮）
	CurrentTurns int `json:"current_turns"`
	// TotalTurns 会话累计轮数
	TotalTurns int `json:"total_turns"`
	// TurnsThreshold 增量轮数触发阈值（按上下文窗口分档；0 = 未启用轮数触发）
	TurnsThreshold int `json:"turns_threshold"`
	// CompressedCount 已压缩进摘要的累计消息条数（无摘要为 0）
	CompressedCount int `json:"compressed_count"`
	// SummaryType 摘要类型：llm / raw（LLM 失败降级原文归档）/ 空（无摘要）
	SummaryType string `json:"summary_type"`
	// LastCompressedCount 最近一次压缩并入摘要的消息条数（手动压缩响应专用；0 = 本次未实际压缩）
	LastCompressedCount int `json:"last_compressed_count"`
	// Attention 是否达到任一触发阈值（token 超阈值或增量轮数达阈值），前端高亮提示
	Attention bool `json:"attention"`
}

// KnowledgeChatRequest 快速问答请求（单步 RAG 管道）
// AgentID：请求级智能体标识（id_str 或数字主键均可），非空时覆盖会话绑定，
// 供 resolveAgentConfig 解析本次对话使用的配置（模型/检索参数等）
// KnowledgeIDs/TagIDs/MCPServiceIDs/SkillNames/MentionedItems/WebSearchEnabled：
// 前端已发送的输入状态字段，当前仅记录到会话 last_request_state 快照（供前端恢复），
// 暂不参与检索过滤（检索范围由 KnowledgeBaseIDs 决定）
type KnowledgeChatRequest struct {
	Query            string   `json:"query"`
	KnowledgeBaseIDs []string `json:"knowledge_base_ids"`
	KnowledgeIDs     []string `json:"knowledge_ids,omitempty"`
	// AgentID 请求级智能体标识：兼容数字主键与 id_str 两种形态（前端可能传 number 或 string）
	AgentID          entity.FlexString           `json:"agent_id"`
	TagIDs           []string                    `json:"tag_ids,omitempty"`
	MCPServiceIDs    []string                    `json:"mcp_service_ids,omitempty"`
	SkillNames       []string                    `json:"skill_names,omitempty"`
	MentionedItems   []entity.StateMentionedItem `json:"mentioned_items,omitempty"`
	WebSearchEnabled *bool                       `json:"web_search_enabled,omitempty"`
	Channel          string                      `json:"channel"`
}

// AgentChatRequest 智能推理对话请求（与 KnowledgeChatRequest 字段一致，复用前端入参）
type AgentChatRequest struct {
	Query            string   `json:"query"`
	KnowledgeBaseIDs []string `json:"knowledge_base_ids"`
	KnowledgeIDs     []string `json:"knowledge_ids,omitempty"`
	// AgentID 请求级智能体标识（id_str 或数字主键均可），非空时覆盖会话绑定
	// （前端切换智能体后按当前选择下发，避免会话旧绑定继续生效）；
	// FlexString 兼容前端传数字主键的形态
	AgentID          entity.FlexString           `json:"agent_id"`
	TagIDs           []string                    `json:"tag_ids,omitempty"`
	MCPServiceIDs    []string                    `json:"mcp_service_ids,omitempty"`
	SkillNames       []string                    `json:"skill_names,omitempty"`
	MentionedItems   []entity.StateMentionedItem `json:"mentioned_items,omitempty"`
	WebSearchEnabled *bool                       `json:"web_search_enabled,omitempty"`
	Channel          string                      `json:"channel"`
}

// AgentChatResponse 智能推理对话响应
// Stream 由 controller 消费并映射为 SSE 事件；Collector 收集 kb_search 引用（落库数据源）
type AgentChatResponse struct {
	Stream    *agent.EventStream
	Collector *tools.ResultCollector
}

// CreateSessionRequest 创建会话请求
type CreateSessionRequest struct {
	Title            string              `json:"title"`
	Source           string              `json:"source"`
	KnowledgeBaseIDs []string            `json:"knowledge_base_ids"`
	AgentEnabled     bool                `json:"agent_enabled"`
	AgentID          string              `json:"agent_id,omitempty"` // 关联的 Agent ID（如内置模板 builtin-smart-reasoning）
	AgentConfig      *entity.AgentConfig `json:"agent_config,omitempty"`
}

// UpdateSessionRequest 更新会话请求
type UpdateSessionRequest struct {
	Title            *string  `json:"title,omitempty"`
	Description      *string  `json:"description,omitempty"`
	KnowledgeBaseIDs []string `json:"knowledge_base_ids,omitempty"`
	AgentEnabled     *bool    `json:"agent_enabled,omitempty"`
	AgentID          *string  `json:"agent_id,omitempty"`
	SummaryModelID   *string  `json:"summary_model_id,omitempty"`
}
