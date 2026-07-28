package models

import (
	"docmind/internal/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册模型相关路由
func (ctrl *Controller) RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/models/providers", middleware.Auth(), ctrl.ListProviders)

	modelGroup := r.Group("/models")
	modelGroup.Use(middleware.Auth())
	{
		modelGroup.GET("", ctrl.ListModels)
		modelGroup.POST("", ctrl.CreateModel)
		modelGroup.GET("/docmindcloud/status", ctrl.GetDocMindCloudStatus)
		modelGroup.GET("/:id", ctrl.GetModel)
		modelGroup.PUT("/:id", ctrl.UpdateModel)
		modelGroup.DELETE("/:id", ctrl.DeleteModel)
		modelGroup.PUT("/:id/credentials", ctrl.PutModelCredentials)
		modelGroup.DELETE("/:id/credentials/:field", ctrl.DeleteModelCredentialField)
		modelGroup.POST("/:id/debug", ctrl.DebugModel)
	}

	docMindCloudGroup := r.Group("/docmindcloud")
	docMindCloudGroup.Use(middleware.Auth())
	{
		docMindCloudGroup.POST("/credentials", ctrl.SaveDocMindCloudCredentials)
	}
}
