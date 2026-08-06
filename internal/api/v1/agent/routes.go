package agent

import (
	"docmind/internal/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册智能体路由
func (ctrl *Controller) RegisterRoutes(r *gin.RouterGroup) {
	group := r.Group("/")
	group.Use(middleware.Auth())
	{
		group.GET("agents", ctrl.ListAgents)
		group.POST("agents", ctrl.CreateAgent)
		group.GET("agents/:id", ctrl.GetAgent)
		group.PUT("agents/:id", ctrl.UpdateAgent)
		group.DELETE("agents/:id", ctrl.DeleteAgent)
		group.POST("agents/:id/copy", ctrl.CopyAgent)
		group.DELETE("agents/:id/override", ctrl.ResetAgentOverride)
	}
}
