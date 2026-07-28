package initialization

import (
	"docmind/internal/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册初始化/模型探测路由
func (ctrl *Controller) RegisterRoutes(r *gin.RouterGroup) {
	group := r.Group("/initialization")
	group.Use(middleware.Auth())
	{
		group.POST("/remote/check", ctrl.CheckRemoteModel)
		group.POST("/embedding/test", ctrl.TestEmbeddingModel)
		group.POST("/rerank/check", ctrl.CheckRerankModel)
		group.POST("/asr/check", ctrl.CheckASRModel)

		group.GET("/ollama/status", ctrl.GetOllamaStatus)
		group.GET("/ollama/models", ctrl.ListOllamaModels)
		group.POST("/ollama/models/check", ctrl.CheckOllamaModels)
		group.POST("/ollama/models/download", ctrl.DownloadOllamaModel)
		group.GET("/ollama/download/progress/:taskId", ctrl.GetOllamaDownloadProgress)
		group.GET("/ollama/download/tasks", ctrl.ListOllamaDownloadTasks)
	}
}
