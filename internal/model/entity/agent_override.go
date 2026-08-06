package entity

// AgentOverride 用户对内置智能体的个性化覆盖（agents 表保持全局模板纯净）
// 读取时：内置模板为基底，叠加当前用户的覆盖；"恢复默认" = 删除覆盖行
type AgentOverride struct {
	BaseEntity
	UserID      uint        `gorm:"index:idx_override_user_agent,unique;not null;comment:覆盖归属用户ID" json:"user_id"`
	AgentID     string      `gorm:"type:varchar(128);index:idx_override_user_agent,unique;not null;comment:内置智能体ID(agents.id_str)" json:"agent_id"`
	Name        string      `gorm:"type:varchar(255);not null;default:'';comment:覆盖后的名称(空=沿用模板)" json:"name"`
	Description string      `gorm:"type:text;comment:覆盖后的描述" json:"description"`
	Avatar      string      `gorm:"type:varchar(512);comment:覆盖后的头像" json:"avatar"`
	Config      AgentConfig `gorm:"type:json;comment:覆盖后的智能体配置(全量)" json:"config"`
}

// TableName 指定表名
func (AgentOverride) TableName() string {
	return "agent_overrides"
}
