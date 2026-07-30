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
			kb.POST("knowledge/file", ctrl.UploadKnowledgeFile)
			kb.GET("knowledge", ctrl.ListKnowledge)

			// FAQ
			kb.GET("faq/entries", ctrl.ListFAQEntries)
			kb.GET("faq/entries/export", ctrl.ExportFAQEntries)
			kb.POST("faq/entry", ctrl.CreateFAQEntry)
			kb.POST("faq/entries", ctrl.BatchUpsertFAQEntries)
			kb.PUT("faq/entries/fields", ctrl.BatchUpdateFAQFields)
			kb.PUT("faq/entries/:entryId", ctrl.UpdateFAQEntry)
			kb.DELETE("faq/entries", ctrl.DeleteFAQEntries)

			// Tag
			kb.GET("tags", ctrl.ListTags)
			kb.POST("tags", ctrl.CreateTag)
			kb.PUT("tags/:tagId", ctrl.UpdateTag)
			kb.DELETE("tags/:tagId", ctrl.DeleteTag)
		}

		// 独立路由 — knowledge 和 chunks 不挂在 :id 下
		group.GET("knowledge/:id", ctrl.GetKnowledge)
		group.POST("knowledge/:id/reparse", ctrl.ReparseKnowledge)
		group.DELETE("knowledge/:id", ctrl.DeleteKnowledge)
		group.PUT("knowledge/tags", ctrl.UpdateKnowledgeTags)
		group.GET("chunks/:id", ctrl.ListKnowledgeChunks)
	}
}
