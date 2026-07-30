package chat

import (
	"docmind/internal/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册会话与对话路由
func (ctrl *Controller) RegisterRoutes(r *gin.RouterGroup) {
	group := r.Group("/")
	group.Use(middleware.Auth())
	{
		// 会话 CRUD
		group.POST("sessions", ctrl.CreateSession)
		group.GET("sessions", ctrl.ListSessions)
		group.DELETE("sessions/batch", ctrl.BatchDeleteSessions)

		// 单个会话操作
		group.GET("sessions/:id", ctrl.GetSession)
		group.PUT("sessions/:id", ctrl.UpdateSession)
		group.DELETE("sessions/:id", ctrl.DeleteSession)
		group.POST("sessions/:id/pin", ctrl.PinSession)
		group.DELETE("sessions/:id/pin", ctrl.UnpinSession)
		group.POST("sessions/:id/stop", ctrl.StopSession)
		group.DELETE("sessions/:id/messages", ctrl.ClearSessionMessages)
		group.POST("sessions/:id/generate_title", ctrl.GenerateTitle)

		// 消息历史
		group.GET("messages/:session_id/load", ctrl.LoadMessages)

		// 对话（SSE 流式）
		group.POST("knowledge-chat/:session_id", ctrl.KnowledgeChat)
		group.POST("agent-chat/:session_id", ctrl.AgentChat)
	}
}
