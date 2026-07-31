package app

import (
	"context"
	docreaderclient "docmind/pkg/docreader/client"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"docmind/internal/api"
	"docmind/internal/repository"
	"docmind/internal/service"
	"docmind/pkg/config"
	"docmind/pkg/database"
	"docmind/pkg/logger"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// App 应用结构体
type App struct {
	cfg             *config.Config
	pgDB            *gorm.DB
	redis           *redis.Client
	docReaderClient *docreaderclient.Client
	docReaderCmd    *exec.Cmd
	router          *api.Router
	server          *http.Server
}

// NewApp 创建应用实例
func NewApp() *App {
	return &App{}
}

// Initialize 初始化应用
func (a *App) Initialize() error {
	// 1. 加载配置
	if err := a.initConfig(); err != nil {
		return err
	}

	// 2. 初始化日志
	if err := a.initLogger(); err != nil {
		return err
	}

	// 3. 初始化数据库
	if err := a.initDatabase(); err != nil {
		return err
	}

	// 4. 初始化依赖
	if err := a.initDependencies(); err != nil {
		return err
	}

	// 5. 初始化路由
	a.initRouter()

	// 6. 初始化服务器
	a.initServer()

	return nil
}

// initConfig 加载配置
func (a *App) initConfig() error {
	cfg, err := config.Load("")
	if err != nil {
		return fmt.Errorf("加载配置失败: %w", err)
	}
	a.cfg = cfg
	return nil
}

// initLogger 初始化日志
func (a *App) initLogger() error {
	if err := logger.Init(&a.cfg.Log); err != nil {
		return fmt.Errorf("日志初始化失败: %w", err)
	}

	// 打印启动横幅
	logger.Info("=========================================")
	logger.Info(fmt.Sprintf("欢迎使用 %s", a.cfg.App.Name))
	logger.Info(fmt.Sprintf("版本: %s", a.cfg.App.Version))
	logger.Info(fmt.Sprintf("模式: %s", a.cfg.App.Mode))
	logger.Info("配置加载成功")
	logger.Info("=========================================")

	return nil
}

// startDocReader 自动启动 DocReader Python gRPC 微服务
func (a *App) startDocReader() {
	// 获取项目根目录
	workDir, err := os.Getwd()
	if err != nil {
		logger.Warn("获取工作目录失败，无法启动 DocReader", zap.Error(err))
		return
	}
	docReaderDir := filepath.Join(workDir, "pkg", "docreader")
	venvPython := filepath.Join(docReaderDir, ".venv", "Scripts", "python.exe")

	// 检查 .venv 中的 python 是否存在，否则用系统 python
	pythonBin := "python"
	if _, err := os.Stat(venvPython); err == nil {
		pythonBin = venvPython
	}

	cmd := exec.Command(pythonBin, "main.py")
	cmd.Dir = docReaderDir
	// 设置 PYTHONPATH 使 docreader 包可被发现
	cmd.Env = append(os.Environ(),
		"OCR_BACKEND=no_ocr",
		"PYTHONPATH="+filepath.Join(workDir, "pkg"),
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	logger.Info("启动 DocReader 微服务...",
		zap.String("python", pythonBin),
		zap.String("dir", docReaderDir),
	)

	if err := cmd.Start(); err != nil {
		logger.Warn("DocReader 微服务启动失败，文件解析不可用", zap.Error(err))
		return
	}

	a.docReaderCmd = cmd
	logger.Info("DocReader 微服务已启动", zap.Int("pid", cmd.Process.Pid))
}

// initDatabase 初始化数据库
func (a *App) initDatabase() error {
	// 初始化 PostgreSQL
	pgDB, err := database.InitPostgreSQL(&a.cfg.Database.PostgreSQL)
	if err != nil {
		return fmt.Errorf("PostgreSQL 初始化失败: %w", err)
	}
	a.pgDB = pgDB

	if err := a.pgDB.Exec("CREATE EXTENSION IF NOT EXISTS vector").Error; err != nil {
		logger.Warn("启用 pgvector 扩展失败", zap.Error(err))
	}

	// 自动迁移数据库表
	logger.Info("开始数据库迁移...")
	if err := a.pgDB.AutoMigrate(
	//&entity.KnowledgeBase{},
	//&entity.Knowledge{},
	//&entity.Chunk{},
	//&entity.Tag{},
	//&entity.FAQ{},
	//&entity.VectorStore{},
	//&entity.ChunkVector{},
	//&entity.Model{},
	//&entity.Session{},
	//&entity.Message{},
	//&entity.Agent{},
	); err != nil {
		logger.Warn("数据库迁移警告", zap.Error(err))
	} else {
		logger.Info("数据库迁移完成")
	}

	// 初始化 Redis（可选）
	rs, err := database.InitRedis(&a.cfg.Database.Redis)
	if err != nil {
		logger.Warn("Redis 初始化失败，将不影响核心功能", zap.Error(err))
	}
	a.redis = rs

	return nil
}

// initDependencies 初始化依赖注入
func (a *App) initDependencies() error {

	// 自动启动 DocReader Python 微服务
	a.startDocReader()

	var docReaderCli *docreaderclient.Client
	if addr := a.cfg.DocReader.Addr; addr != "" {
		// 等待 Python 服务就绪，最多重试 5 次
		var client *docreaderclient.Client
		var err error
		for i := 0; i < 5; i++ {
			if i > 0 {
				time.Sleep(1 * time.Second)
			}
			client, err = docreaderclient.NewClient(addr)
			if err == nil {
				break
			}
			logger.Info("等待 DocReader 服务就绪...",
				zap.Int("attempt", i+1),
				zap.Error(err),
			)
		}
		if err != nil {
			logger.Warn("DocReader 初始化失败，知识文件解析接口将不可用",
				zap.String("addr", addr),
				zap.Error(err),
			)
		} else {
			docReaderCli = client
		}
	}
	a.docReaderClient = docReaderCli

	// 创建 Repository
	userRepo := repository.NewUserRepository(a.pgDB)
	refreshTokenRepo := repository.NewRefreshTokenRepository(a.redis)
	vectorStoreRepo := repository.NewVectorStoreRepository(a.pgDB)
	modelRepo := repository.NewModelRepository(a.pgDB)
	systemSettingRepo := repository.NewSystemSettingRepository(a.pgDB)
	knowledgeBaseRepo := repository.NewKnowledgeBaseRepository(a.pgDB)
	knowledgeRepo := repository.NewKnowledgeRepository(a.pgDB)
	faqRepo := repository.NewFAQRepository(a.pgDB)
	tagRepo := repository.NewTagRepository(a.pgDB)
	chunkRepo := repository.NewChunkRepository(a.pgDB)

	// 创建 Service
	userSvc := service.NewUserService(userRepo)
	authSvc := service.NewAuthService(userRepo, refreshTokenRepo, userSvc)
	modelSvc := service.NewModelService(modelRepo, systemSettingRepo)
	chunkerSvc := service.NewChunkerService()
	vectorStoreSvc := service.NewVectorStoreService(vectorStoreRepo, knowledgeBaseRepo, chunkRepo, modelSvc, a.pgDB, a.cfg)
	imageStorageSvc, err := service.NewImageStorageService(a.cfg.MinIO)
	if err != nil {
		logger.Warn("图片存储服务初始化失败，文档图片将保留原始引用", zap.Error(err))
		imageStorageSvc = service.NewNoopImageStorageService()
	}
	pipelineGateway := service.NewKnowledgePipelineGatewayMock()
	knowledgeBaseSvc := service.NewKnowledgeBaseService(a.pgDB, knowledgeBaseRepo, knowledgeRepo, faqRepo, tagRepo, vectorStoreRepo, pipelineGateway)
	faqSvc := service.NewFAQService(a.pgDB, knowledgeBaseRepo, faqRepo)
	tagSvc := service.NewTagService(a.pgDB, knowledgeBaseRepo, tagRepo, faqRepo)
	knowledgeSvc := service.NewKnowledgeService(knowledgeRepo, knowledgeBaseRepo, chunkRepo, a.docReaderClient, imageStorageSvc, a.cfg)

	// 会话与对话相关
	sessionRepo := repository.NewSessionRepository(a.pgDB)
	messageRepo := repository.NewMessageRepository(a.pgDB)
	chatModelFactory := service.NewChatModelFactory(modelRepo)
	embedderFactory := service.NewEmbedderFactory(modelRepo)
	agentRepo := repository.NewAgentRepository(a.pgDB)
	chatSvc, err := service.NewChatService(sessionRepo, messageRepo, chatModelFactory, embedderFactory, knowledgeBaseRepo, vectorStoreRepo, agentRepo, a.pgDB)
	if err != nil {
		return fmt.Errorf("创建 ChatService 失败: %w", err)
	}

	// 智能体
	agentSvc := service.NewAgentService(agentRepo)
	// 确保内置智能体存在
	if err := service.SeedBuiltinAgents(agentRepo); err != nil {
		logger.Warn("创建内置智能体失败", zap.Error(err))
	}

	// 创建 Router
	a.router = api.NewRouter(userSvc, authSvc, chunkerSvc, knowledgeSvc, vectorStoreSvc, modelSvc, knowledgeBaseSvc, faqSvc, tagSvc, chatSvc, agentSvc)

	return nil
}

// initRouter 初始化路由
func (a *App) initRouter() {
	// 设置 Gin 模式
	gin.SetMode(a.cfg.App.Mode)
}

// initServer 初始化 HTTP 服务器
func (a *App) initServer() {
	engine := gin.New()

	// 注册路由
	a.router.Setup(engine)

	// 创建 HTTP 服务器（SSE 流式响应需要较长超时）
	a.server = &http.Server{
		Addr:           fmt.Sprintf(":%d", a.cfg.App.Port),
		Handler:        engine,
		ReadTimeout:    60 * time.Second,
		WriteTimeout:   300 * time.Second, // 5 分钟，SSE 流式响应需要
		MaxHeaderBytes: 1 << 20,           // 1 MB
	}
}

// Run 运行应用
func (a *App) Run() {
	// 启动 HTTP 服务器
	go func() {
		logger.Info("HTTP 服务器启动",
			zap.String("addr", a.server.Addr),
			zap.String("mode", a.cfg.App.Mode),
		)
		if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("HTTP 服务器启动失败", zap.Error(err))
		}
	}()

	// 优雅关闭
	a.gracefulShutdown()
}

// gracefulShutdown 优雅关闭
func (a *App) gracefulShutdown() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("正在关闭服务器...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 关闭 HTTP 服务器
	if err := a.server.Shutdown(ctx); err != nil {
		logger.Error("服务器关闭失败", zap.Error(err))
	}

	// 关闭数据库连接
	_ = database.ClosePostgreSQL()
	_ = database.CloseRedis()

	// 同步日志
	_ = logger.Sync()

	logger.Info("服务器已关闭")
	logger.Info("=========================================")
}
