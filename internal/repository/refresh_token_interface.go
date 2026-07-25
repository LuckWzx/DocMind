package repository

import "time"

// RefreshTokenRepository 刷新令牌仓储接口
type RefreshTokenRepository interface {
	// Save 保存刷新令牌，附带 TTL 自动过期
	Save(token string, userID uint, ttl time.Duration) error
	// FindByToken 根据令牌查找用户 ID
	FindByToken(token string) (uint, error)
	// Delete 删除指定令牌
	Delete(token string) error
	// DeleteByUserID 删除用户的所有刷新令牌
	DeleteByUserID(userID uint) error
}
