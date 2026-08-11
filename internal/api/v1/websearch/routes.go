package websearch

import (
	"docmind/internal/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册网页搜索提供方路由
// 注意：/types 静态路由必须在 /:id 之前注册（gin 路由匹配）
func (ctrl *Controller) RegisterRoutes(r *gin.RouterGroup) {
	group := r.Group("/web-search-providers")
	group.Use(middleware.Auth())
	{
		group.GET("/types", ctrl.Types)
		group.POST("/test", ctrl.TestRaw)
		group.GET("", ctrl.List)
		group.POST("", ctrl.Create)
		group.GET("/:id", ctrl.Get)
		group.PUT("/:id", ctrl.Update)
		group.DELETE("/:id", ctrl.Delete)
		group.POST("/:id/test", ctrl.TestProvider)
		// 凭据子资源（密钥独立管理，响应脱敏）
		group.PUT("/:id/credentials", ctrl.UpdateCredentials)
		group.DELETE("/:id/credentials/:field", ctrl.DeleteCredential)
	}
}
