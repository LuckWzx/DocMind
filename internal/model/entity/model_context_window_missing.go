package entity

// ModelContextWindowMissing 上下文大小缺失记录：
// 当模型厂商不提供元数据查询接口且内置映射表未命中时写入，
// 供后续定期补足内置映射表（context_window 确定后自动清理）。
// 注意：本表删除必须释放 model_id 唯一索引，保证后续同一模型可再次写入。
type ModelContextWindowMissing struct {
	BaseEntity
	UserID    uint   `gorm:"index;not null;comment:模型所属用户ID" json:"user_id"`
	ModelID   uint   `gorm:"uniqueIndex;not null;comment:模型ID" json:"model_id"`
	ModelName string `gorm:"type:varchar(255);not null;comment:模型名称" json:"model_name"`
	Provider  string `gorm:"type:varchar(64);default:'';comment:模型厂商" json:"provider"`
	BaseURL   string `gorm:"type:varchar(512);default:'';comment:接口地址" json:"base_url"`
	Source    string `gorm:"type:varchar(32);default:'';comment:模型来源" json:"source"`
	Type      string `gorm:"type:varchar(32);default:'';comment:模型类型" json:"type"`
	Reason    string `gorm:"type:text;comment:缺失原因（接口错误信息或未命中说明）" json:"reason"`
}

// TableName 指定表名
func (ModelContextWindowMissing) TableName() string {
	return "model_context_window_missing"
}
