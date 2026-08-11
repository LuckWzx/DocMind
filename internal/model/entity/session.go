package entity

import (
	"database/sql/driver"
	"encoding/json"
)

// StateMentionedItem 会话请求状态快照中的 @ 提及目标（知识库/文件/标签/MCP/技能）。
// 与 message.go 的 MentionedItem（uint ID，消息落库用）区分：此处对齐前端
// mentioned_items 载荷的字符串 ID 形态（web/src/stores/settings.ts SessionLastRequestStatePayload）
type StateMentionedItem struct {
	ID        string `json:"id"`
	Name      string `json:"name,omitempty"`
	Type      string `json:"type"`
	KbType    string `json:"kb_type,omitempty"`
	KBID      string `json:"kb_id,omitempty"`
	KBName    string `json:"kb_name,omitempty"`
	ServiceID string `json:"service_id,omitempty"`
	SkillName string `json:"skill_name,omitempty"`
}

// SessionLastRequestState 会话最近一次对话请求的状态快照（JSON 存储）。
// 前端打开历史会话时据此恢复输入栏（智能体/模型/知识库/提及项等），
// 字段全部可选——历史会话或新建会话首发前的请求没有这条记录。
// 字段对齐前端 SessionLastRequestStatePayload。
type SessionLastRequestState struct {
	AgentID          string               `json:"agent_id,omitempty"`
	AgentEnabled     *bool                `json:"agent_enabled,omitempty"`
	ModelID          string               `json:"model_id,omitempty"`
	KnowledgeBaseIDs []string             `json:"knowledge_base_ids,omitempty"`
	KnowledgeIDs     []string             `json:"knowledge_ids,omitempty"`
	TagIDs           []string             `json:"tag_ids,omitempty"`
	MCPServiceIDs    []string             `json:"mcp_service_ids,omitempty"`
	SkillNames       []string             `json:"skill_names,omitempty"`
	MentionedItems   []StateMentionedItem `json:"mentioned_items,omitempty"`
	WebSearchEnabled *bool                `json:"web_search_enabled,omitempty"`
}

// Scan 实现 sql.Scanner
func (s *SessionLastRequestState) Scan(value interface{}) error {
	return scanJSONStruct(value, s)
}

// Value 实现 driver.Valuer
func (s SessionLastRequestState) Value() (driver.Value, error) {
	return jsonStructValue(s)
}

// StringSlice 自定义类型，让 GORM 正确处理 JSON 字符串数组
type StringSlice []string

// Scan 实现 sql.Scanner
func (s *StringSlice) Scan(value interface{}) error {
	if value == nil {
		*s = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	if len(bytes) == 0 {
		*s = nil
		return nil
	}
	return json.Unmarshal(bytes, s)
}

// Value 实现 driver.Valuer
func (s StringSlice) Value() (driver.Value, error) {
	if s == nil {
		return nil, nil
	}
	return json.Marshal(s)
}

// Session 会话，一次连续的问答对话
type Session struct {
	BaseEntity
	UserID           uint                     `gorm:"index;not null;comment:所属用户ID" json:"user_id"`
	Title            string                   `gorm:"type:varchar(255);comment:会话标题" json:"title"`
	Description      string                   `gorm:"type:text;comment:会话描述" json:"description"`
	Source           string                   `gorm:"type:varchar(32);default:'web';comment:来源 web/im/embed/api" json:"source"`
	Pinned           bool                     `gorm:"default:false;comment:是否置顶" json:"pinned"`
	AgentID          string                   `gorm:"type:varchar(128);index;comment:关联的Agent ID" json:"agent_id,omitempty"`
	KnowledgeBaseIDs StringSlice              `gorm:"type:json;comment:关联的知识库ID列表" json:"knowledge_base_ids"`
	AgentEnabled     bool                     `gorm:"default:false;comment:是否启用Agent模式" json:"agent_enabled"`
	AgentConfig      *AgentConfig             `gorm:"type:json;comment:Agent配置" json:"agent_config,omitempty"`
	SummaryModelID   string                   `gorm:"type:varchar(128);comment:摘要模型ID" json:"summary_model_id,omitempty"`
	LastRequestState *SessionLastRequestState `gorm:"type:json;comment:最近一次请求状态快照(前端恢复输入栏用)" json:"last_request_state,omitempty"`
	LastMessage      string                   `gorm:"type:text;comment:最后一条消息预览" json:"last_message,omitempty"`
	MessageCount     int                      `gorm:"default:0;comment:消息数量" json:"message_count"`

	// 关联（不存入数据库）
	Messages []Message `gorm:"foreignKey:SessionID" json:"-"`
}

// TableName 指定表名
func (Session) TableName() string {
	return "sessions"
}
