package entity

// User 用户实体
type User struct {
	BaseEntity
	Username string `gorm:"type:varchar(50);uniqueIndex;not null;column:username" json:"username"`
	Password string `gorm:"type:varchar(255);not null;column:password" json:"-"`
	Email    string `gorm:"type:varchar(100);uniqueIndex;column:email" json:"email"`
	Nickname string `gorm:"type:varchar(50);column:nickname" json:"nickname"`
	Avatar   string `gorm:"type:varchar(255);column:avatar" json:"avatar"`
	Status   int    `gorm:"type:smallint;default:1;column:status" json:"status"` // 1:正常, 2:禁用
}

// TableName 指定表名
func (User) TableName() string {
	return "users"
}
