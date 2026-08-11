package entity

// WebSearchProvider 网页搜索提供方配置（按用户隔离，每个用户的配置仅自己可见可用）
type WebSearchProvider struct {
	BaseEntity
	UserID      uint   `gorm:"index;not null;comment:所属用户ID" json:"user_id"`
	Name        string `gorm:"type:varchar(128);not null;comment:提供方名称" json:"name"`
	Provider    string `gorm:"type:varchar(32);not null;comment:引擎类型 duckduckgo/tavily/baidu" json:"provider"`
	Description string `gorm:"type:text;comment:描述" json:"description"`
	APIKey      string `gorm:"type:varchar(512);comment:API密钥(仅落库不响应)" json:"-"`
	BaseURL     string `gorm:"type:varchar(512);comment:自建服务地址(可选,如自托管SearXNG)" json:"-"`
	ProxyURL    string `gorm:"type:varchar(512);comment:代理地址(可选,如DuckDuckGo需代理访问)" json:"-"`
	ExtraConfig JSON   `gorm:"type:jsonb;comment:扩展配置" json:"-"`
	IsDefault   bool   `gorm:"default:false;comment:是否默认提供方" json:"is_default"`
	IsEnabled   bool   `gorm:"default:true;comment:是否启用" json:"is_enabled"`
}

// TableName 指定表名
func (WebSearchProvider) TableName() string {
	return "web_search_providers"
}
