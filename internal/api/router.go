package api

import (
	"docmind/internal/api/v1/agent"
	"docmind/internal/api/v1/auth"
	"docmind/internal/api/v1/chat"
	"docmind/internal/api/v1/chunker"
	"docmind/internal/api/v1/files"
	"docmind/internal/api/v1/initialization"
	"docmind/internal/api/v1/knowledge"
	"docmind/internal/api/v1/knowledgebase"
	"docmind/internal/api/v1/mcp"
	"docmind/internal/api/v1/models"
	"docmind/internal/api/v1/tag"
	"docmind/internal/api/v1/vectorstore"
	"docmind/internal/api/v1/websearch"
	"docmind/internal/memory/longterm"
	"docmind/internal/middleware"
	"docmind/internal/service"
	"docmind/pkg/config"
	"docmind/pkg/sse"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "docmind/docs" // Swagger 文档
)

// Router 路由
type Router struct {
	authCtrl           *auth.Controller
	chunkerCtrl        *chunker.Controller
	knowledgeCtrl      *knowledge.Controller
	vectorStoreCtrl    *vectorstore.Controller
	knowledgeBaseCtrl  *knowledgebase.Controller
	modelCtrl          *models.Controller
	initializationCtrl *initialization.Controller
	chatCtrl           *chat.Controller
	agentCtrl          *agent.Controller
	tagCtrl            *tag.Controller
	mcpCtrl            *mcp.Controller
	webSearchCtrl      *websearch.Controller
	filesCtrl          *files.Controller
}

// NewRouter 创建路由
func NewRouter(
	userService service.UserService,
	authService service.AuthService,
	chunkerService service.ChunkerService,
	knowledgeService service.KnowledgeService,
	vectorStoreService service.VectorStoreService,
	modelService service.ModelService,
	knowledgeBaseService service.KnowledgeBaseService,
	faqService service.FAQService,
	tagService service.TagService,
	chatService service.ChatService,
	agentService service.AgentService,
	mcpService service.MCPServiceService,
	webSearchService service.WebSearchService,
	sseRegistry *sse.Registry,
	redis *redis.Client,
	sseCfg config.SSEConfig,
	memorySvc longterm.MemoryService,
	storageRoot string, // 本地存储根目录（config.storage.local_root，默认 data/files）
) *Router {
	return &Router{
		authCtrl:           auth.NewController(authService, userService),
		chunkerCtrl:        chunker.NewController(chunkerService),
		knowledgeCtrl:      knowledge.NewController(knowledgeService, knowledgeBaseService, faqService),
		vectorStoreCtrl:    vectorstore.NewController(vectorStoreService),
		knowledgeBaseCtrl:  knowledgebase.NewController(knowledgeBaseService, faqService, tagService),
		modelCtrl:          models.NewController(modelService),
		initializationCtrl: initialization.NewController(modelService),
		chatCtrl:           chat.NewController(chatService, sseRegistry, redis, sseCfg, memorySvc, chat.NewRunRegistry()),
		agentCtrl:          agent.NewController(agentService),
		tagCtrl:            tag.NewController(tagService),
		mcpCtrl:            mcp.NewController(mcpService),
		webSearchCtrl:      websearch.NewController(webSearchService),
		filesCtrl:          files.NewController(storageRoot),
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

	// 本地文件代理（渲染 Markdown 图片：沙箱图表等；前端带 token fetch → blob URL）
	engine.GET("/files", middleware.Auth(), r.filesCtrl.Serve)

	// API v1 路由组
	v1 := engine.Group("/api/v1")
	{
		// 认证路由
		r.authCtrl.RegisterRoutes(v1)

		// 模型管理路由
		r.modelCtrl.RegisterRoutes(v1)

		// 初始化 / 模型探测路由
		r.initializationCtrl.RegisterRoutes(v1)

		// 分块预览路由
		r.chunkerCtrl.RegisterRoutes(v1)

		// 知识库文件导入路由
		r.knowledgeCtrl.RegisterRoutes(v1)

		// 向量存储路由
		r.vectorStoreCtrl.RegisterRoutes(v1)

		// 知识库管理路由
		r.knowledgeBaseCtrl.RegisterRoutes(v1)

		// 标签路由
		r.tagCtrl.RegisterRoutes(v1)

		// 会话与对话路由
		r.chatCtrl.RegisterRoutes(v1)

		// 智能体路由
		r.agentCtrl.RegisterRoutes(v1)

		// MCP 服务路由
		r.mcpCtrl.RegisterRoutes(v1)

		// 网页搜索提供方路由
		r.webSearchCtrl.RegisterRoutes(v1)
	}
}
