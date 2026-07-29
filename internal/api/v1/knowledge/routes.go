package knowledge

import (
	"docmind/internal/middleware"

	"github.com/gin-gonic/gin"
)

func (ctrl *Controller) RegisterRoutes(r *gin.RouterGroup) {
	group := r.Group("/")
	group.Use(middleware.Auth())
	{
		group.GET("knowledge-bases", ctrl.ListKnowledgeBases)
		group.POST("knowledge-bases", ctrl.CreateKnowledgeBase)
		group.GET("knowledge-bases/:id", ctrl.GetKnowledgeBase)
		group.PUT("knowledge-bases/:id", ctrl.UpdateKnowledgeBase)
		group.DELETE("knowledge-bases/:id", ctrl.DeleteKnowledgeBase)

		group.POST("knowledge-bases/:kbId/knowledge/file", ctrl.UploadKnowledgeFile)
		group.GET("knowledge-bases/:kbId/knowledge", ctrl.ListKnowledge)
		group.GET("knowledge/:id", ctrl.GetKnowledge)
		group.POST("knowledge/:id/reparse", ctrl.ReparseKnowledge)
		group.DELETE("knowledge/:id", ctrl.DeleteKnowledge)
		group.PUT("knowledge/tags", ctrl.UpdateKnowledgeTags)
		group.GET("chunks/:id", ctrl.ListKnowledgeChunks)

		group.GET("knowledge-bases/:kbId/faq/entries", ctrl.ListFAQEntries)
		group.POST("knowledge-bases/:kbId/faq/entry", ctrl.CreateFAQEntry)
		group.PUT("knowledge-bases/:kbId/faq/entries/:entryId", ctrl.UpdateFAQEntry)
		group.POST("knowledge-bases/:kbId/faq/entries", ctrl.BatchUpsertFAQEntries)
		group.PUT("knowledge-bases/:kbId/faq/entries/fields", ctrl.BatchUpdateFAQFields)
		group.DELETE("knowledge-bases/:kbId/faq/entries", ctrl.DeleteFAQEntries)
		group.GET("knowledge-bases/:kbId/faq/entries/export", ctrl.ExportFAQEntries)

		group.GET("knowledge-bases/:kbId/tags", ctrl.ListTags)
		group.POST("knowledge-bases/:kbId/tags", ctrl.CreateTag)
		group.PUT("knowledge-bases/:kbId/tags/:tagId", ctrl.UpdateTag)
		group.DELETE("knowledge-bases/:kbId/tags/:tagId", ctrl.DeleteTag)
	}
}
