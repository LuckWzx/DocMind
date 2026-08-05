package entity

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// ============================================================================
// Message 配置依赖
// ============================================================================

// MessageImages 消息附带的图片列表
type MessageImages []MessageImage

// Scan implements sql.Scanner interface for database deserialization
func (m *MessageImages) Scan(value interface{}) error {
	if value == nil {
		*m = make(MessageImages, 0)
		return nil
	}
	var b []byte
	switch v := value.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		*m = make(MessageImages, 0)
		return fmt.Errorf("unsupported type: %T", value)
	}
	return json.Unmarshal(b, m)
}

// Value implements driver.Valuer interface for database serialization
func (m MessageImages) Value() (driver.Value, error) {
	if m == nil {
		return json.Marshal([]MessageImage{})
	}
	return json.Marshal(m)
}

// MessageImage 消息中的单张图片
type MessageImage struct {
	URL     string `json:"url"`               // 图片URL
	Caption string `json:"caption,omitempty"` // 图片描述（OCR/VLM生成的文本）
}

// Reference 引用来源（检索到的 chunk，运行时填充，不存库）
type Reference struct {
	ChunkID        uint    `json:"chunk_id"`
	Content        string  `json:"content"`
	Score          float64 `json:"score"`
	KnowledgeID    uint    `json:"knowledge_id"`
	KnowledgeTitle string  `json:"knowledge_title"`
}

// MentionedItem @提及的知识库/文件
type MentionedItem struct {
	Type string `json:"type"` // knowledge_base / file
	ID   uint   `json:"id"`
	Name string `json:"name,omitempty"`
}

// ReferenceJSON 用于数据库JSON存储的引用结构
type ReferenceJSON struct {
	ChunkID        uint    `json:"chunk_id"`
	Content        string  `json:"content"`
	Score          float64 `json:"score"`
	KnowledgeID    uint    `json:"knowledge_id"`
	KnowledgeTitle string  `json:"knowledge_title"`
}

// ReferenceJSONs 引用列表的JSON序列化类型
type ReferenceJSONs []ReferenceJSON

// Scan implements sql.Scanner interface for database deserialization
func (r *ReferenceJSONs) Scan(value interface{}) error {
	if value == nil {
		*r = make(ReferenceJSONs, 0)
		return nil
	}
	var b []byte
	switch v := value.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		*r = make(ReferenceJSONs, 0)
		return fmt.Errorf("unsupported type: %T", value)
	}
	return json.Unmarshal(b, r)
}

// Value implements driver.Valuer interface for database serialization
func (r ReferenceJSONs) Value() (driver.Value, error) {
	if r == nil {
		return json.Marshal([]ReferenceJSON{})
	}
	return json.Marshal(r)
}

// ============================================================================
// Agent 执行步骤相关类型（参考 WeKnora 实现）
// ============================================================================

// AgentStepToolResult 工具调用结果
type AgentStepToolResult struct {
	Success bool        `json:"success"`
	Output  string      `json:"output,omitempty"`
	Error   string      `json:"error,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// AgentStepToolCall 单次工具调用
type AgentStepToolCall struct {
	ID       string               `json:"id"`
	Name     string               `json:"name"`
	Args     interface{}          `json:"args,omitempty"`
	Result   *AgentStepToolResult `json:"result,omitempty"`
	Duration int64                `json:"duration,omitempty"`
}

// AgentStep Agent 执行的单个步骤（一次 LLM 推理 + 可选工具调用）
type AgentStep struct {
	Iteration        int                 `json:"iteration"`
	Timestamp        time.Time           `json:"timestamp"`
	Duration         int64               `json:"duration,omitempty"`
	Thought          string              `json:"thought,omitempty"`
	ReasoningContent string              `json:"reasoning_content,omitempty"`
	ToolCalls        []AgentStepToolCall `json:"tool_calls,omitempty"`
}

// AgentSteps Agent 执行步骤列表
type AgentSteps []AgentStep

// Value implements driver.Valuer interface for database serialization
func (a AgentSteps) Value() (driver.Value, error) {
	if a == nil {
		return json.Marshal([]AgentStep{})
	}
	return json.Marshal(a)
}

// Scan implements sql.Scanner interface for database deserialization
func (a *AgentSteps) Scan(value interface{}) error {
	if value == nil {
		*a = make(AgentSteps, 0)
		return nil
	}
	var b []byte
	switch v := value.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		*a = make(AgentSteps, 0)
		return fmt.Errorf("unsupported type: %T", value)
	}
	return json.Unmarshal(b, a)
}

// Message 消息，会话中的一条对话记录
type Message struct {
	BaseEntity
	SessionID           uint            `gorm:"index;comment:所属会话ID" json:"session_id"`
	Role                string          `gorm:"type:varchar(16);not null;comment:角色 user/assistant/system" json:"role"`
	Content             string          `gorm:"type:text;comment:消息正文" json:"content"`
	RenderedContent     string          `gorm:"type:text;comment:渲染给前端的正文" json:"rendered_content"`
	Images              MessageImages   `gorm:"type:json;comment:消息附带的图片" json:"images"`
	ReferencesJSON      *ReferenceJSONs `gorm:"type:json;comment:引用的知识来源" json:"-"`
	KnowledgeReferences []Reference     `gorm:"-;comment:引用的知识来源 运行时填充" json:"knowledge_references"`
	FinishReason        string          `gorm:"type:varchar(32);comment:LLM完成原因" json:"finish_reason"`

	// Agent 执行相关字段（参考 WeKnora 实现）
	AgentSteps      AgentSteps `gorm:"type:jsonb;column:agent_steps;default:'[]'" json:"agent_steps,omitempty"`
	IsCompleted     bool       `gorm:"type:boolean;column:is_completed;default:false" json:"is_completed"`
	AgentDurationMs int64      `gorm:"type:bigint;column:agent_duration_ms;default:0" json:"agent_duration_ms,omitempty"`
	IsFallback      bool       `gorm:"type:boolean;column:is_fallback;default:false" json:"is_fallback,omitempty"`

	// 关联（不存入数据库）
	Session Session `gorm:"foreignKey:SessionID" json:"-"`
}

// AfterFind GORM hook: 读取数据库后将 ReferencesJSON 转换为 KnowledgeReferences
func (m *Message) AfterFind(tx *gorm.DB) error {
	if m.ReferencesJSON != nil && len(*m.ReferencesJSON) > 0 {
		m.KnowledgeReferences = make([]Reference, 0, len(*m.ReferencesJSON))
		for _, r := range *m.ReferencesJSON {
			m.KnowledgeReferences = append(m.KnowledgeReferences, Reference{
				ChunkID:        r.ChunkID,
				Content:        r.Content,
				Score:          r.Score,
				KnowledgeID:    r.KnowledgeID,
				KnowledgeTitle: r.KnowledgeTitle,
			})
		}
	}
	return nil
}

// TableName 指定表名
func (Message) TableName() string {
	return "messages"
}
