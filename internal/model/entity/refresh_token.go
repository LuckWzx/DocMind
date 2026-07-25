package entity

import "time"

// RefreshToken 刷新令牌实体
type RefreshToken struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    uint      `gorm:"column:user_id;not null" json:"user_id"`
	Token     string    `gorm:"type:varchar(512);uniqueIndex;not null;column:token" json:"-"`
	ExpiresAt time.Time `gorm:"column:expires_at;not null" json:"expires_at"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
}

// TableName 指定表名
func (RefreshToken) TableName() string {
	return "refresh_tokens"
}
