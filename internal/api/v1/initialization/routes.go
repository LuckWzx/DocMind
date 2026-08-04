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
		group.POST("/remote/check", handleModelTest(ctrl.modelService.CheckRemoteModel))
		group.POST("/embedding/test", handleModelTest(ctrl.modelService.TestEmbeddingModel))
		group.POST("/rerank/check", handleModelTest(ctrl.modelService.CheckRerankModel))
		group.POST("/asr/check", handleModelTest(ctrl.modelService.CheckASRModel))

		group.GET("/ollama/status", ctrl.GetOllamaStatus)
		group.GET("/ollama/models", ctrl.ListOllamaModels)
		group.POST("/ollama/models/check", ctrl.CheckOllamaModels)
		group.POST("/ollama/models/download", ctrl.DownloadOllamaModel)
		group.GET("/ollama/download/progress/:taskId", ctrl.GetOllamaDownloadProgress)
		group.GET("/ollama/download/tasks", ctrl.ListOllamaDownloadTasks)
	}
}
