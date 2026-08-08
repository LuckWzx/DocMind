package service

import (
	"context"
	"time"

	"docmind/internal/model/entity"
	"docmind/internal/pipeline"

	"github.com/cloudwego/eino/schema"
)

// ChatService 对话服务接口
type ChatService interface {
	// KnowledgeChat 单步 RAG 对话，返回流式响应
	KnowledgeChat(ctx context.Context, sessionID uint, userID uint, req *KnowledgeChatRequest, stepCallback pipeline.StepCallback) (*schema.StreamReader[*schema.Message], []VectorSearchResult, error)
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
	// StopChat 停止对话
	StopChat(ctx context.Context, sessionID uint, userID uint, messageID string) error
	// ClearSessionMessages 清空会话消息
	ClearSessionMessages(ctx context.Context, sessionID uint, userID uint) error
	// LoadMessages 加载历史消息
	LoadMessages(ctx context.Context, sessionID uint, userID uint, limit int, beforeTime *time.Time) ([]*entity.Message, error)
	// SaveAssistantMessage 保存助手消息
	SaveAssistantMessage(ctx context.Context, sessionID uint, content string, references []entity.Reference, agentSteps entity.AgentSteps, isCompleted bool, agentDurationMs int64, isFallback bool) error
	// GenerateSessionTitle 若会话标题仍为默认占位（"新对话"），则用首条用户消息生成标题并落库，返回新标题。已被用户手动重命名的会话不会被覆盖。
	GenerateSessionTitle(ctx context.Context, sessionID uint, userID uint, query string) (string, error)
}

// KnowledgeChatRequest 知识问答请求
type KnowledgeChatRequest struct {
	Query            string   `json:"query"`
	KnowledgeBaseIDs []string `json:"knowledge_base_ids"`
	Channel          string   `json:"channel"`
}

// CreateSessionRequest 创建会话请求
type CreateSessionRequest struct {
	Title            string              `json:"title"`
	Source           string              `json:"source"`
	KnowledgeBaseIDs []string            `json:"knowledge_base_ids"`
	AgentEnabled     bool                `json:"agent_enabled"`
	AgentID          string              `json:"agent_id"` // 关联的智能体 ID
	AgentConfig      *entity.AgentConfig `json:"agent_config,omitempty"`
}

// UpdateSessionRequest 更新会话请求
type UpdateSessionRequest struct {
	Title            *string  `json:"title,omitempty"`
	Description      *string  `json:"description,omitempty"`
	KnowledgeBaseIDs []string `json:"knowledge_base_ids,omitempty"`
	AgentEnabled     *bool    `json:"agent_enabled,omitempty"`
	SummaryModelID   *string  `json:"summary_model_id,omitempty"`
}
