package entity

// WebSearchProvider 网页搜索提供方配置
type WebSearchProvider struct {
	BaseEntity
	Name      string `gorm:"type:varchar(255);not null;comment:提供方名称" json:"name"`
	Engine    string `gorm:"type:varchar(32);not null;comment:引擎类型" json:"engine"`
	APIKey    string `gorm:"type:varchar(512);comment:API密钥" json:"api_key"`
	IsEnabled bool   `gorm:"default:true;comment:是否启用" json:"is_enabled"`
}

// TableName 指定表名
func (WebSearchProvider) TableName() string {
	return "web_search_providers"
}
