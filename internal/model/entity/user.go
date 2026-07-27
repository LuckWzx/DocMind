package entity

// User 用户实体
type User struct {
	BaseEntity
	Username string `gorm:"type:varchar(50);uniqueIndex;not null;column:username;comment:用户名" json:"username"`
	Password string `gorm:"type:varchar(255);not null;column:password;comment:密码" json:"-"`
	Email    string `gorm:"type:varchar(100);uniqueIndex;column:email;comment:邮箱" json:"email"`
	Nickname string `gorm:"type:varchar(50);column:nickname;comment:昵称" json:"nickname"`
	Avatar   string `gorm:"type:varchar(255);column:avatar;comment:头像" json:"avatar"`
	Status   int    `gorm:"type:smallint;default:1;column:status;comment:状态 1:正常 2:禁用" json:"status"`
}

// TableName 指定表名
func (User) TableName() string {
	return "users"
}
