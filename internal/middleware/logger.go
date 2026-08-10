package middleware

import (
	"net/http"
	"strings"
	"time"

	"docmind/pkg/logger"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// slowRequestThreshold 慢请求阈值：正常请求超过该耗时则升级为 Warn，便于发现性能瓶颈
const slowRequestThreshold = 1000 * time.Millisecond

// Logger 日志中间件
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 跳过 Swagger 静态资源请求，避免日志刷屏
		if strings.HasPrefix(c.Request.URL.Path, "/swagger/") {
			c.Next()
			return
		}
		// 开始时间
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		// 处理请求
		c.Next()

		// 结束时间
		latency := time.Since(start)
		status := c.Writer.Status()

		// 记录日志
		fields := []zap.Field{
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.String("query", query),
			zap.Int("status", status),
			zap.String("ip", c.ClientIP()),
			zap.String("user_agent", c.Request.UserAgent()),
			zap.Duration("latency", latency),
		}

		// 获取错误信息
		if len(c.Errors) > 0 {
			fields = append(fields, zap.String("errors", c.Errors.String()))
		}

		// 按状态码与耗时分级记录，避免正常请求刷屏（默认 info 级别下 Debug 不输出）：
		// - 5xx           -> Error：服务端错误，必须记录
		// - 其他 4xx       -> Warn：业务/权限问题（404 多为前端探测性请求，归为 Debug）
		// - 正常但超时      -> Warn：慢请求，便于发现性能瓶颈
		// - 其余（2xx/3xx/404）-> Debug：需排查时可将日志级别调为 debug 恢复全量
		switch {
		case status >= 500:
			logger.Error("HTTP Request", fields...)
		case status >= 400 && status != http.StatusNotFound:
			logger.Warn("HTTP Request", fields...)
		case latency > slowRequestThreshold:
			logger.Warn("Slow HTTP Request", fields...)
		default:
			logger.Debug("HTTP Request", fields...)
		}
	}
}
