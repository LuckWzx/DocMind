package knowledgebase

import (
	"docmind/internal/middleware"

	"github.com/gin-gonic/gin"
)

func (ctrl *Controller) RegisterRoutes(r *gin.RouterGroup) {
	group := r.Group("/")
	group.Use(middleware.Auth())
	{
		// 知识库 CRUD
		group.GET("knowledge-bases", ctrl.ListKnowledgeBases)
		group.POST("knowledge-bases", ctrl.CreateKnowledgeBase)

		// 知识库子资源 — 用嵌套分组避免 Gin 路由冲突
		kb := group.Group("knowledge-bases/:id")
		{
			kb.GET("", ctrl.GetKnowledgeBase)
			kb.PUT("", ctrl.UpdateKnowledgeBase)
			kb.DELETE("", ctrl.DeleteKnowledgeBase)

			// 知识文件
			//kb.POST("knowledge/file", ctrl.UploadKnowledgeFile)
		}

	}
}
