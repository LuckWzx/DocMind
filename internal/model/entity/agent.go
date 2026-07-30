package entity

import (
	"database/sql/driver"
)

// AgentConfig 智能体配置（JSON 存储，字段对齐前端 CustomAgentConfig）
type AgentConfig struct {
	// 基础设置
	AgentMode    string `json:"agent_mode"` // quick-answer / smart-reasoning
	SystemPrompt string `json:"system_prompt"`

	// 模型设置
	ModelID             string   `json:"model_id"`
	RerankModelID       string   `json:"rerank_model_id"`
	Temperature         *float64 `json:"temperature,omitempty"`
	MaxCompletionTokens *int     `json:"max_completion_tokens,omitempty"`

	// 知识库设置
	KnowledgeBases []string `json:"knowledge_bases"`

	// 检索策略
	EmbeddingTopK   int     `json:"embedding_top_k"`
	VectorThreshold float64 `json:"vector_threshold"`
	RerankTopK      int     `json:"rerank_top_k"`

	// 多轮对话
	MultiTurnEnabled bool `json:"multi_turn_enabled"`
	HistoryTurns     int  `json:"history_turns"`

	// 网络搜索
	WebSearchEnabled bool `json:"web_search_enabled"`
}

// Scan 实现 sql.Scanner
func (c *AgentConfig) Scan(value interface{}) error {
	return scanJSONStruct(value, c)
}

// Value 实现 driver.Valuer
func (c AgentConfig) Value() (driver.Value, error) {
	return jsonStructValue(c)
}

// Agent 智能体
type Agent struct {
	BaseEntity
	UserID      uint        `gorm:"index;default:0;not null;comment:所属用户ID(0=内置)" json:"user_id"`
	IDStr       string      `gorm:"type:varchar(128);uniqueIndex;not null;comment:字符串ID" json:"id_str"`
	Name        string      `gorm:"type:varchar(255);not null;comment:名称" json:"name"`
	Description string      `gorm:"type:text;comment:描述" json:"description"`
	Avatar      string      `gorm:"type:varchar(512);comment:头像" json:"avatar"`
	IsBuiltin   bool        `gorm:"default:false;comment:是否内置" json:"is_builtin"`
	Config      AgentConfig `gorm:"type:json;comment:智能体配置" json:"config"`
}

// TableName 指定表名
func (Agent) TableName() string {
	return "agents"
}
