package middleware

import (
	"net/http"
	"time"

	"docmind/pkg/logger"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Idempotency Redis SETNX 幂等去重中间件。
//
// 前端每次请求携带 X-Request-ID，同一 ID 在 TTL 内重复提交返回 409，
// 防止网络超时重试导致 LLM 双倍调用（双倍费用、双条消息）。
// Redis 不可用或请求无 X-Request-ID 时放行（fail-open），不阻塞业务。
func Idempotency(rdb *redis.Client, ttl time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		reqID := c.GetHeader("X-Request-ID")
		if reqID == "" || rdb == nil || ttl <= 0 {
			c.Next()
			return
		}
		ok, err := rdb.SetNX(c.Request.Context(), "sse:idem:"+reqID, 1, ttl).Result()
		if err != nil {
			logger.Warn("幂等去重检查失败，放行",
				zap.String("request_id", reqID),
				zap.Error(err))
			c.Next()
			return
		}
		if !ok {
			logger.Warn("检测到重复请求", zap.String("request_id", reqID))
			c.JSON(http.StatusConflict, gin.H{
				"success": false,
				"code":    "DUPLICATE_REQUEST",
				"message": "重复请求，请勿重复提交",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}
