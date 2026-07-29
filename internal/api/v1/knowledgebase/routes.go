package knowledgebase

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册知识库文件上传路由
func (ctrl *Controller) RegisterRoutes(r *gin.RouterGroup) {
	knowledgeBaseGroup := r.Group("/knowledge-bases")
	//knowledgeBaseGroup.Use(middleware.Auth())
	{
		knowledgeBaseGroup.POST("/:id/knowledge/file", ctrl.UploadKnowledgeFile)
	}
}
