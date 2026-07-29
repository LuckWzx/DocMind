package vectorstore

import (
	"docmind/internal/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册向量存储路由
func (ctrl *Controller) RegisterRoutes(r *gin.RouterGroup) {
	vectorStoreGroup := r.Group("/vector-stores")
	vectorStoreGroup.Use(middleware.Auth())
	{
		vectorStoreGroup.GET("", ctrl.List)
		vectorStoreGroup.POST("", ctrl.Create)
		vectorStoreGroup.GET("/:id", ctrl.Get)
		vectorStoreGroup.PUT("/:id", ctrl.Update)
		vectorStoreGroup.DELETE("/:id", ctrl.Delete)
		vectorStoreGroup.POST("/:id/test-connection", ctrl.TestConnection)
		vectorStoreGroup.POST("/:id/search", ctrl.Search)
		vectorStoreGroup.POST("/:id/knowledge-bases/:knowledge_base_id/index", ctrl.IndexKnowledgeBase)
	}
}
