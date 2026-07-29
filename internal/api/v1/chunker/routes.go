package chunker

import (
	"docmind/internal/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册分块预览路由
func (ctrl *Controller) RegisterRoutes(r *gin.RouterGroup) {
	group := r.Group("/chunker")
	group.Use(middleware.Auth())
	{
		group.POST("/preview", ctrl.PreviewChunking)
	}
}
