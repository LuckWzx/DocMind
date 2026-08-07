package entity

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// EmbeddingParameters Embedding 专属参数
type EmbeddingParameters struct {
	Dimension                 int  `json:"dimension,omitempty"`
	TruncatePromptTokens      int  `json:"truncate_prompt_tokens,omitempty"`
	SupportsDimensionOverride bool `json:"supports_dimension_override,omitempty"`
}

// ModelParameters 模型参数（JSON/JSONB 存储）
type ModelParameters struct {
	BaseURL             string              `json:"base_url,omitempty"`
	APIKey              string              `json:"api_key,omitempty"`
	AppID               string              `json:"app_id,omitempty"`
	AppSecret           string              `json:"app_secret,omitempty"`
	APIVersion          string              `json:"api_version,omitempty"`
	ModelName           string              `json:"model_name,omitempty"`
	Provider            string              `json:"provider,omitempty"`
	InterfaceType       string              `json:"interface_type,omitempty"`
	ParameterSize       string              `json:"parameter_size,omitempty"`
	Temperature         float64             `json:"temperature,omitempty"`
	MaxTokens           int                 `json:"max_tokens,omitempty"`
	ContextWindow       int                 `json:"context_window,omitempty"`
	Dimension           int                 `json:"dimension,omitempty"`
	KeepAlive           string              `json:"keep_alive,omitempty"`
	EmbeddingParameters EmbeddingParameters `json:"embedding_parameters,omitempty"`
	ExtraConfig         map[string]string   `json:"extra_config,omitempty"`
	CustomHeaders       map[string]string   `json:"custom_headers,omitempty"`
	SupportsVision      bool                `json:"supports_vision,omitempty"`
	MaxConcurrency      int                 `json:"max_concurrency,omitempty"`
}

// Value 实现 driver.Valuer
func (p ModelParameters) Value() (driver.Value, error) {
	raw, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	return string(raw), nil
}

// Scan 实现 sql.Scanner
func (p *ModelParameters) Scan(value interface{}) error {
	if value == nil {
		*p = ModelParameters{}
		return nil
	}

	switch v := value.(type) {
	case []byte:
		if len(v) == 0 {
			*p = ModelParameters{}
			return nil
		}
		return json.Unmarshal(v, p)
	case string:
		if v == "" {
			*p = ModelParameters{}
			return nil
		}
		return json.Unmarshal([]byte(v), p)
	default:
		return fmt.Errorf("unsupported ModelParameters scan type: %T", value)
	}
}

// Model 系统可用的 AI 模型配置
type Model struct {
	BaseEntity
	UserID      uint            `gorm:"index;default:0;not null;comment:所属用户ID" json:"user_id"`
	Name        string          `gorm:"type:varchar(255);not null;comment:模型名称" json:"name"`
	DisplayName string          `gorm:"type:varchar(255);default:'';comment:展示名称" json:"display_name"`
	Type        string          `gorm:"type:varchar(32);not null;comment:模型类型" json:"type"`
	Source      string          `gorm:"type:varchar(32);not null;comment:来源" json:"source"`
	Description string          `gorm:"type:text;comment:描述" json:"description"`
	Status      string          `gorm:"type:varchar(32);default:'active';comment:状态" json:"status"`
	IsDefault   bool            `gorm:"default:false;comment:是否默认模型" json:"is_default"`
	IsBuiltin   bool            `gorm:"default:false;comment:是否内置模型" json:"is_builtin"`
	Parameters  ModelParameters `gorm:"type:json;comment:模型参数" json:"parameters"`
}

// TableName 指定表名
func (Model) TableName() string {
	return "models"
}

// 模型类型常量
const (
	ModelTypeEmbedding   = "Embedding"   // Embedding 模型
	ModelTypeRerank      = "Rerank"      // Rerank 模型
	ModelTypeKnowledgeQA = "KnowledgeQA" // 对话模型（知识问答）
	ModelTypeVLLM        = "VLLM"        // 多模态视觉模型
	ModelTypeASR         = "ASR"         // 语音识别模型
)

// 模型状态常量
const (
	ModelStatusActive         = "active"
	ModelStatusDownloading    = "downloading"
	ModelStatusDownloadFailed = "download_failed"
)
