package entity

// ModelParameters 模型参数（JSONB 存储）
type ModelParameters struct {
	BaseURL     string  `json:"base_url"`
	APIKey      string  `json:"api_key"`
	APIVersion  string  `json:"api_version,omitempty"`
	ModelName   string  `json:"model_name"`
	Temperature float64 `json:"temperature,omitempty"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
	Dimension   int     `json:"dimension,omitempty"`  // Embedding 维度
	KeepAlive   string  `json:"keep_alive,omitempty"` // Ollama 保活时间
}

// Model 系统可用的 AI 模型配置
type Model struct {
	BaseEntity
	Name       string          `gorm:"type:varchar(255);not null;comment:模型名称" json:"name"`
	Type       string          `gorm:"type:varchar(32);not null;comment:模型类型" json:"type"`
	Source     string          `gorm:"type:varchar(32);not null;comment:来源" json:"source"`
	Status     string          `gorm:"type:varchar(32);default:'active';comment:状态" json:"status"`
	Parameters ModelParameters `gorm:"type:json;comment:模型参数" json:"parameters"`
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
)

// 模型状态常量
const (
	ModelStatusActive         = "active"
	ModelStatusDownloading    = "downloading"
	ModelStatusDownloadFailed = "download_failed"
)
