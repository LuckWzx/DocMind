package knowledge

import (
	"docmind/internal/middleware"
	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册知识库文件上传路由
func (ctrl *Controller) RegisterRoutes(r *gin.RouterGroup) {
	r.Use(middleware.Auth())
	// 独立路由 — knowledge 和 chunks 不挂在 :id 下
	r.GET("knowledge/:id", ctrl.GetKnowledge)
	r.POST("knowledge/:id/reparse", ctrl.ReparseKnowledge)
	r.DELETE("knowledge/:id", ctrl.DeleteKnowledge)
	r.PUT("knowledge/tags", ctrl.UpdateKnowledgeTags)
	r.GET("chunks/:id", ctrl.ListKnowledgeChunks)
	knowledgeBaseGroup := r.Group("/knowledge-bases")
	{
		//knowledgeBaseGroup.POST("/:id/knowledge/file", ctrl.UploadKnowledgeFile)
		// 知识库子资源 — 用嵌套分组避免 Gin 路由冲突
		kb := knowledgeBaseGroup.Group(":id")
		{
			kb.POST("/:id/knowledge/file", ctrl.UploadKnowledgeFile)
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
	}
}
