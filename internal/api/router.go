package api

import (
	"docmind/internal/api/v1/auth"
	"docmind/internal/api/v1/initialization"
	"docmind/internal/api/v1/models"
	"docmind/internal/api/v1/user"
	"docmind/internal/middleware"
	"docmind/internal/service"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "docmind/docs" // Swagger 文档
)

// Router 路由
type Router struct {
	userCtrl           *user.Controller
	authCtrl           *auth.Controller
	modelCtrl          *models.Controller
	initializationCtrl *initialization.Controller
}

// NewRouter 创建路由
func NewRouter(
	userService service.UserService,
	authService service.AuthService,
	modelService service.ModelService,
) *Router {
	return &Router{
		userCtrl:           user.NewController(userService),
		authCtrl:           auth.NewController(authService, userService),
		modelCtrl:          models.NewController(modelService),
		initializationCtrl: initialization.NewController(modelService),
	}
}

// Setup 设置路由
func (r *Router) Setup(engine *gin.Engine) {
	// 全局中间件
	engine.Use(middleware.Recovery())
	engine.Use(middleware.Logger())
	engine.Use(middleware.CORS())

	// 健康检查
	engine.GET("/api/v1/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "ok",
			"message": "DocMind API is running",
		})
	})

	// Swagger 文档（仅非 production 环境启用）
	if gin.Mode() != gin.ReleaseMode {
		engine.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler,
			ginSwagger.DefaultModelsExpandDepth(-1),
			ginSwagger.DocExpansion("list"),
			ginSwagger.DeepLinking(true),
			ginSwagger.PersistAuthorization(true),
		))
	}

	// API v1 路由组
	v1 := engine.Group("/api/v1")
	{
		// 认证路由
		r.authCtrl.RegisterRoutes(v1)

		// 用户路由
		r.userCtrl.RegisterRoutes(v1)

		// 模型管理路由
		r.modelCtrl.RegisterRoutes(v1)

		// 初始化 / 模型探测路由
		r.initializationCtrl.RegisterRoutes(v1)
	}
}
