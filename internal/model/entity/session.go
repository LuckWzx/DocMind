package entity

import (
	"database/sql/driver"
	"encoding/json"
)

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
	UserID           uint         `gorm:"index;not null;comment:所属用户ID" json:"user_id"`
	Title            string       `gorm:"type:varchar(255);comment:会话标题" json:"title"`
	Description      string       `gorm:"type:text;comment:会话描述" json:"description"`
	Source           string       `gorm:"type:varchar(32);default:'web';comment:来源 web/im/embed/api" json:"source"`
	Pinned           bool         `gorm:"default:false;comment:是否置顶" json:"pinned"`
	AgentID          string       `gorm:"type:varchar(128);index;comment:关联的Agent ID" json:"agent_id,omitempty"`
	KnowledgeBaseIDs StringSlice  `gorm:"type:json;comment:关联的知识库ID列表" json:"knowledge_base_ids"`
	AgentEnabled     bool         `gorm:"default:false;comment:是否启用Agent模式" json:"agent_enabled"`
	AgentConfig      *AgentConfig `gorm:"type:json;comment:Agent配置" json:"agent_config,omitempty"`
	SummaryModelID   string       `gorm:"type:varchar(128);comment:摘要模型ID" json:"summary_model_id,omitempty"`
	LastMessage      string       `gorm:"type:text;comment:最后一条消息预览" json:"last_message,omitempty"`
	MessageCount     int          `gorm:"default:0;comment:消息数量" json:"message_count"`

	// 关联（不存入数据库）
	Messages []Message `gorm:"foreignKey:SessionID" json:"-"`
}

// TableName 指定表名
func (Session) TableName() string {
	return "sessions"
}
