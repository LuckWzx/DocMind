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
		// 工具审批设置（用户级偏好：可对可见服务（含全局）的每个工具设置人工审批）
		mcpGroup.GET("/:id/tool-approvals", ctrl.GetToolApprovals)
		mcpGroup.PUT("/:id/tool-approvals/:toolName", ctrl.SetToolApproval)
	}
}
