package repository

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	refreshTokenPrefix  = "refresh_token:"       // refresh_token:{token} -> userID
	refreshTokenUserSet = "refresh_tokens:user:" // refresh_tokens:user:{userID} -> set of tokens
)

// refreshTokenRepository 使用 Redis 的刷新令牌仓储实现
type refreshTokenRepository struct {
	rdb *redis.Client
}

// NewRefreshTokenRepository 创建刷新令牌仓储
func NewRefreshTokenRepository(rdb *redis.Client) RefreshTokenRepository {
	return &refreshTokenRepository{rdb: rdb}
}

// Save 保存刷新令牌
func (r *refreshTokenRepository) Save(token string, userID uint, ttl time.Duration) error {
	ctx := context.Background()
	key := refreshTokenPrefix + token
	userKey := refreshTokenUserSet + strconv.FormatUint(uint64(userID), 10)

	// 使用 Pipeline 批量执行，减少网络往返
	pipe := r.rdb.Pipeline()
	pipe.Set(ctx, key, userID, ttl) // 存储 token -> userID，带 TTL
	pipe.SAdd(ctx, userKey, token)  // 记录该用户的所有 token
	pipe.Expire(ctx, userKey, ttl)  // user set 也设置过期时间
	_, err := pipe.Exec(ctx)
	return err
}

// FindByToken 根据令牌查找用户 ID
func (r *refreshTokenRepository) FindByToken(token string) (uint, error) {
	ctx := context.Background()
	key := refreshTokenPrefix + token

	val, err := r.rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return 0, nil // token 不存在
	}
	if err != nil {
		return 0, fmt.Errorf("查询 refresh token 失败: %w", err)
	}

	userID, err := strconv.ParseUint(val, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("解析 user_id 失败: %w", err)
	}

	return uint(userID), nil
}

// Delete 删除指定令牌
func (r *refreshTokenRepository) Delete(token string) error {
	ctx := context.Background()

	// 先查出 token 对应的 userID，以便从 user set 中移除
	userID, err := r.FindByToken(token)
	if err != nil {
		return err
	}

	pipe := r.rdb.Pipeline()
	pipe.Del(ctx, refreshTokenPrefix+token)
	if userID > 0 {
		userKey := refreshTokenUserSet + strconv.FormatUint(uint64(userID), 10)
		pipe.SRem(ctx, userKey, token)
	}
	_, err = pipe.Exec(ctx)
	return err
}

// DeleteByUserID 删除用户的所有刷新令牌
func (r *refreshTokenRepository) DeleteByUserID(userID uint) error {
	ctx := context.Background()
	userKey := refreshTokenUserSet + strconv.FormatUint(uint64(userID), 10)

	// 获取该用户的所有 token
	tokens, err := r.rdb.SMembers(ctx, userKey).Result()
	if err != nil {
		return fmt.Errorf("获取用户 token 列表失败: %w", err)
	}

	if len(tokens) == 0 {
		return nil
	}

	// 批量删除
	pipe := r.rdb.Pipeline()
	for _, t := range tokens {
		pipe.Del(ctx, refreshTokenPrefix+t)
	}
	pipe.Del(ctx, userKey)
	_, err = pipe.Exec(ctx)
	return err
}
