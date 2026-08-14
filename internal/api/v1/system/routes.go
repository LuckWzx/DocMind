package system

import (
	"docmind/internal/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册系统信息路由
func (ctrl *Controller) RegisterRoutes(r *gin.RouterGroup) {
	group := r.Group("/system")
	group.Use(middleware.Auth())
	{
		group.GET("/info", ctrl.Info)
	}
}
