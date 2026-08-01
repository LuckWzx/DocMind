package tag

import (
	"docmind/internal/middleware"
	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册标签路由
func (ctrl *Controller) RegisterRoutes(r *gin.RouterGroup) {
	r.Use(middleware.Auth())
	kb := r.Group("/knowledge-bases/:id")
	{
		kb.GET("tags", ctrl.ListTags)
		kb.POST("tags", ctrl.CreateTag)
		kb.PUT("tags/:tagId", ctrl.UpdateTag)
		kb.DELETE("tags/:tagId", ctrl.DeleteTag)
	}
}
