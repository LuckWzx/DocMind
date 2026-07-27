package auth

import (
	"docmind/internal/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册认证路由
func (ctrl *Controller) RegisterRoutes(r *gin.RouterGroup) {
	authGroup := r.Group("/auth")
	{
		// 公开接口
		authGroup.POST("/register", ctrl.Register)
		authGroup.POST("/login", ctrl.Login)
		authGroup.POST("/refresh", ctrl.RefreshToken)

		// 需要认证的接口
		authGroup.GET("/me", middleware.Auth(), ctrl.GetCurrentUser)
		authGroup.POST("/logout", middleware.Auth(), ctrl.Logout)
	}
}
