package mcp

import (
	"docmind/internal/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册 MCP 服务路由
func (ctrl *Controller) RegisterRoutes(r *gin.RouterGroup) {
	mcpGroup := r.Group("/mcp-services")
	mcpGroup.Use(middleware.Auth())
	{
		mcpGroup.GET("", ctrl.List)
		mcpGroup.POST("", ctrl.Create)
		mcpGroup.GET("/:id", ctrl.Get)
		mcpGroup.PUT("/:id", ctrl.Update)
		mcpGroup.DELETE("/:id", ctrl.Delete)
		mcpGroup.POST("/:id/test", ctrl.Test)
		mcpGroup.GET("/:id/tools", ctrl.Tools)
		mcpGroup.GET("/:id/resources", ctrl.Resources)
		// 凭据子资源（密钥独立管理，响应脱敏）
		mcpGroup.PUT("/:id/credentials", ctrl.UpdateCredentials)
		mcpGroup.DELETE("/:id/credentials/:field", ctrl.DeleteCredentialField)
		// v1 未实现：tool-approvals / oauth（前端契约保留，路由后置）
	}
}
