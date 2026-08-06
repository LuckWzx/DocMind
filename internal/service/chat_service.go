package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"docmind/internal/llm"
	"docmind/internal/model/entity"
	"docmind/internal/pipeline"
	"docmind/internal/repository"
	bizerrors "docmind/pkg/errors"

	einoschema "github.com/cloudwego/eino/schema"
	"gorm.io/gorm"
)

const (
	defaultTopK        = 5
	defaultTemperature = 0.7

	knowledgeQASystemPrompt = `你是一个知识库问答助手。请根据以下检索到的文档内容回答用户的问题。

规则：
1. 只根据提供的文档内容回答，不要编造信息
2. 如果文档中没有相关信息，请明确告知用户
3. 回答要简洁、准确
4. 在回答中适当引用文档来源`
)

type chatService struct {
	sessionRepo  repository.SessionRepository
	messageRepo  repository.MessageRepository
	modelFactory *llm.ChatModelFactory
	agentSvc     AgentService
	kbRepo       repository.KnowledgeBaseRepository
	ragPipeline  *pipeline.Pipeline
}

// NewChatService 创建对话服务
func NewChatService(
	sessionRepo repository.SessionRepository,
	messageRepo repository.MessageRepository,
	modelFactory *llm.ChatModelFactory,
	embedderFactory *llm.EmbedderFactory,
	rerankerFactory pipeline.PipelineRerankerFactory,
	kbRepo repository.KnowledgeBaseRepository,
	vectorStoreRepo repository.VectorStoreRepository,
	agentSvc AgentService,
	primaryDB *gorm.DB,
) (ChatService, error) {
	// 构建 Pipeline 依赖
	pipelineDeps := &pipeline.PipelineDeps{
		EmbedderFactory: llm.NewPipelineEmbedderFactory(embedderFactory),
		RerankerFactory: rerankerFactory,
		KBRepo:          kbRepo,
		VectorStoreRepo: vectorStoreRepo,
		PrimaryDB:       primaryDB,
		CreateDriver: func(store interface{}) (pipeline.PipelineVectorDriver, func(), error) {
			vs, ok := store.(*entity.VectorStore)
			if !ok {
				return nil, func() {}, fmt.Errorf("无效的向量存储类型")
			}
			db, cleanup, err := resolvePostgresDB(primaryDB, vs)
			if err != nil {
				return nil, func() {}, err
			}
			driver := newPostgresVectorDriver(db, vs)
			return &pipelineVectorDriverAdapter{inner: driver}, cleanup, nil
		},
	}

	// 创建 RAG Pipeline
	ragPipeline, err := pipeline.NewPipeline(pipelineDeps)
	if err != nil {
		return nil, fmt.Errorf("创建 RAG Pipeline 失败: %w", err)
	}

	return &chatService{
		sessionRepo:  sessionRepo,
		messageRepo:  messageRepo,
		modelFactory: modelFactory,
		agentSvc:     agentSvc,
		kbRepo:       kbRepo,
		ragPipeline:  ragPipeline,
	}, nil
}

// KnowledgeChat 单步 RAG 对话
func (s *chatService) KnowledgeChat(ctx context.Context, sessionID uint, userID uint, req *KnowledgeChatRequest) (*einoschema.StreamReader[*einoschema.Message], []VectorSearchResult, error) {
	// 1. 验证 session 归属
	session, err := s.sessionRepo.FindByID(sessionID)
	if err != nil || session == nil {
		return nil, nil, bizerrors.New(bizerrors.CodeResourceNotFound, "会话不存在")
	}
	if session.UserID != userID {
		return nil, nil, bizerrors.New(bizerrors.CodeForbidden, "无权访问该会话")
	}

	// 2. 保存用户消息
	userMsg := &entity.Message{
		SessionID: sessionID,
		Role:      "user",
		Content:   req.Query,
	}
	if err := s.messageRepo.Create(userMsg); err != nil {
		return nil, nil, bizerrors.NewWithErr(bizerrors.CodeInternalError, "保存用户消息失败", err)
	}
	s.sessionRepo.IncrementMessageCount(sessionID)
	s.sessionRepo.UpdateLastMessage(sessionID, req.Query)

	// 3. 从 Agent 解析配置（提前，用于确定历史轮数）
	agentConfig := s.resolveAgentConfig(session, req, userID)

	// 4. 加载最近对话历史（排除刚保存的 userMsg）
	var historyMsgs []*entity.Message
	if agentConfig.MultiTurnEnabled {
		// 每轮 = 1 user + 1 assistant，所以消息数 = 轮数 * 2
		maxHistoryMessages := agentConfig.HistoryTurns * 2
		if maxHistoryMessages <= 0 {
			maxHistoryMessages = 10 // 默认 5 轮 = 10 条消息
		}
		historyMsgs, err = s.messageRepo.ListBySession(sessionID, maxHistoryMessages+1, nil)
		if err != nil {
			historyMsgs = nil // 加载失败不影响对话
		}
	}
	var history []*einoschema.Message
	for _, m := range historyMsgs {
		// 跳过刚保存的 userMsg
		if m.ID == userMsg.ID {
			continue
		}
		role := einoschema.User
		if m.Role == "assistant" {
			role = einoschema.Assistant
		} else if m.Role == "system" {
			role = einoschema.System
		}
		history = append(history, &einoschema.Message{
			Role:    role,
			Content: m.Content,
		})
	}

	// 5. 创建 Pipeline 上下文
	pipelineCtx := &pipeline.Context{
		Query:           req.Query,
		SessionID:       sessionID,
		UserID:          userID,
		AgentConfig:     agentConfig,
		HistoryMessages: history,
		ModelRepo:       s.modelFactory.ModelRepo(),
	}

	// 5. 执行 Pipeline
	result, err := s.ragPipeline.Run(ctx, pipelineCtx)
	if err != nil {
		return nil, nil, bizerrors.NewWithErr(bizerrors.CodeInternalError, "Pipeline 执行失败", err)
	}

	// 6. 转换搜索结果
	var searchResults []VectorSearchResult
	for _, r := range result.RerankedResults {
		searchResults = append(searchResults, VectorSearchResult{
			ChunkID:        r.ChunkID,
			Content:        r.Content,
			Score:          r.Score,
			KnowledgeID:    r.KnowledgeID,
			KnowledgeTitle: r.KnowledgeTitle,
		})
	}

	return result.Stream, searchResults, nil
}

// resolveKBIDs 确定使用的知识库 ID 列表
func (s *chatService) resolveKBIDs(session *entity.Session, reqKBIDs []string) []string {
	if len(reqKBIDs) > 0 {
		return reqKBIDs
	}
	if len(session.KnowledgeBaseIDs) > 0 {
		return []string(session.KnowledgeBaseIDs)
	}
	return nil
}

// resolveChatModelID 确定使用的 Chat 模型 ID
func (s *chatService) resolveChatModelID(session *entity.Session) string {
	// 优先使用 Session 中智能体配置的 model_id
	if session.AgentConfig != nil && session.AgentConfig.ModelID != "" {
		return session.AgentConfig.ModelID
	}
	// 其次使用 Session 的 summary_model_id
	if session.SummaryModelID != "" {
		return session.SummaryModelID
	}
	return "default"
}

// resolveAgentConfig 从 Agent 动态解析配置
func (s *chatService) resolveAgentConfig(session *entity.Session, req *KnowledgeChatRequest, userID uint) *pipeline.AgentConfig {
	config := &pipeline.AgentConfig{
		ModelID:            "default",
		KnowledgeBaseIDs:   req.KnowledgeBaseIDs,
		SystemPrompt:       knowledgeQASystemPrompt,
		Temperature:        defaultTemperature,
		MaxTokens:          2048,
		MultiTurnEnabled:   false,
		HistoryTurns:       5,
		EnableQueryRewrite: false,
		EmbeddingTopK:      defaultTopK,
		VectorThreshold:    0.5,
		RerankTopK:         defaultTopK,
	}

	// 如果请求中没有携带知识库 ID，从 Session 的 AgentConfig 中补充
	// （前端创建会话时会将选中的知识库存入 session.agent_config.knowledge_bases）
	if len(config.KnowledgeBaseIDs) == 0 && session.AgentConfig != nil && len(session.AgentConfig.KnowledgeBases) > 0 {
		config.KnowledgeBaseIDs = session.AgentConfig.KnowledgeBases
	}

	// 如果 Session 关联了 Agent，从 Agent 配置中解析（按用户视角：内置模板 + 用户覆盖）
	if session.AgentID != "" {
		agent, err := s.agentSvc.ResolveForUser(userID, session.AgentID)
		if err == nil && agent != nil {
			if agent.Config.ModelID != "" {
				config.ModelID = agent.Config.ModelID
			}
			// 处理知识库配置
			if agent.Config.KBSelectionMode == "all" {
				// 自动检索所有知识库
				allKBs, err := s.kbRepo.ListByUser(userID)
				if err == nil && len(allKBs) > 0 {
					kbIDs := make([]string, 0, len(allKBs))
					for _, kb := range allKBs {
						kbIDs = append(kbIDs, fmt.Sprintf("%d", kb.ID))
					}
					config.KnowledgeBaseIDs = kbIDs
				}
			} else if len(agent.Config.KnowledgeBases) > 0 {
				config.KnowledgeBaseIDs = agent.Config.KnowledgeBases
			}
			if agent.Config.SystemPrompt != "" {
				config.SystemPrompt = agent.Config.SystemPrompt
			}
			if agent.Config.Temperature != nil {
				config.Temperature = *agent.Config.Temperature
			}
			if agent.Config.MaxCompletionTokens != nil {
				config.MaxTokens = *agent.Config.MaxCompletionTokens
			}
			// 解析查询改写配置
			if agent.Config.EnableRewrite != nil {
				config.EnableQueryRewrite = *agent.Config.EnableRewrite
			}
			if agent.Config.QueryUnderstandModelID != "" {
				config.QueryUnderstandModelID = agent.Config.QueryUnderstandModelID
			}
			if agent.Config.RewritePromptSystem != "" {
				config.RewritePromptSystem = agent.Config.RewritePromptSystem
			}
			if agent.Config.RewritePromptUser != "" {
				config.RewritePromptUser = agent.Config.RewritePromptUser
			}
			// 解析多轮对话配置
			if agent.Config.MultiTurnEnabled != nil {
				config.MultiTurnEnabled = *agent.Config.MultiTurnEnabled
			}
			if agent.Config.HistoryTurns > 0 {
				config.HistoryTurns = agent.Config.HistoryTurns
			}
			// 解析 Rerank 配置
			if agent.Config.RerankModelID != "" {
				config.RerankModelID = agent.Config.RerankModelID
			}
			if agent.Config.RerankTopK > 0 {
				config.RerankTopK = agent.Config.RerankTopK
			}
			if agent.Config.RerankThreshold != nil {
				config.RerankThreshold = *agent.Config.RerankThreshold
			}
			// 解析检索策略配置
			if agent.Config.EmbeddingTopK > 0 {
				config.EmbeddingTopK = agent.Config.EmbeddingTopK
			}
			if agent.Config.VectorThreshold > 0 {
				config.VectorThreshold = agent.Config.VectorThreshold
			}
		}
	}

	// 兜底：如果经过以上所有解析后仍然没有知识库 ID，则自动检索用户的所有知识库
	if len(config.KnowledgeBaseIDs) == 0 {
		if allKBs, err := s.kbRepo.ListByUser(userID); err == nil && len(allKBs) > 0 {
			kbIDs := make([]string, 0, len(allKBs))
			for _, kb := range allKBs {
				kbIDs = append(kbIDs, fmt.Sprintf("%d", kb.ID))
			}
			config.KnowledgeBaseIDs = kbIDs
		}
	}

	return config
}

// resolvePostgresDB 解析 PostgreSQL 连接配置
func resolvePostgresDB(primaryDB *gorm.DB, store *entity.VectorStore) (*gorm.DB, func(), error) {
	connCfg := entity.ConnectionConfig{}
	if err := parseEntityJSON(store.ConnectionConfig, &connCfg); err != nil {
		return nil, nil, fmt.Errorf("解析 connection_config 失败: %w", err)
	}
	if connCfg.UseDefaultConnection || strings.TrimSpace(connCfg.Host) == "" {
		return primaryDB, func() {}, nil
	}
	// 如果配置了自定义连接，创建新的 DB 连接（简化实现：使用默认连接）
	return primaryDB, func() {}, nil
}

// CreateSession 创建会话
func (s *chatService) CreateSession(ctx context.Context, userID uint, req *CreateSessionRequest) (*entity.Session, error) {
	session := &entity.Session{
		UserID:           userID,
		Title:            req.Title,
		Source:           req.Source,
		KnowledgeBaseIDs: req.KnowledgeBaseIDs,
		AgentEnabled:     req.AgentEnabled,
	}
	if req.AgentConfig != nil {
		session.AgentConfig = req.AgentConfig
	}
	if session.Source == "" {
		session.Source = "web"
	}
	if session.Title == "" {
		session.Title = "新对话"
	}
	if err := s.sessionRepo.Create(session); err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalError, "创建会话失败", err)
	}
	return session, nil
}

// GetSession 获取单个会话
func (s *chatService) GetSession(ctx context.Context, sessionID uint, userID uint) (*entity.Session, error) {
	session, err := s.sessionRepo.FindByID(sessionID)
	if err != nil || session == nil {
		return nil, bizerrors.New(bizerrors.CodeResourceNotFound, "会话不存在")
	}
	if session.UserID != userID {
		return nil, bizerrors.New(bizerrors.CodeForbidden, "无权访问该会话")
	}
	return session, nil
}

// ListSessions 获取会话列表
func (s *chatService) ListSessions(ctx context.Context, userID uint, source string, page, pageSize int) ([]*entity.Session, int64, error) {
	return s.sessionRepo.ListByUser(userID, source, page, pageSize)
}

// UpdateSession 更新会话
func (s *chatService) UpdateSession(ctx context.Context, sessionID uint, userID uint, req *UpdateSessionRequest) error {
	session, err := s.sessionRepo.FindByID(sessionID)
	if err != nil || session == nil {
		return bizerrors.New(bizerrors.CodeResourceNotFound, "会话不存在")
	}
	if session.UserID != userID {
		return bizerrors.New(bizerrors.CodeForbidden, "无权操作")
	}
	if req.Title != nil {
		session.Title = *req.Title
	}
	if req.Description != nil {
		session.Description = *req.Description
	}
	if req.KnowledgeBaseIDs != nil {
		session.KnowledgeBaseIDs = req.KnowledgeBaseIDs
	}
	if req.AgentEnabled != nil {
		session.AgentEnabled = *req.AgentEnabled
	}
	if req.SummaryModelID != nil {
		session.SummaryModelID = *req.SummaryModelID
	}
	return s.sessionRepo.Update(session)
}

// DeleteSession 删除会话
func (s *chatService) DeleteSession(ctx context.Context, sessionID uint, userID uint) error {
	session, err := s.sessionRepo.FindByID(sessionID)
	if err != nil || session == nil {
		return bizerrors.New(bizerrors.CodeResourceNotFound, "会话不存在")
	}
	if session.UserID != userID {
		return bizerrors.New(bizerrors.CodeForbidden, "无权操作")
	}
	if err := s.messageRepo.DeleteBySession(sessionID); err != nil {
		return bizerrors.NewWithErr(bizerrors.CodeInternalError, "删除消息失败", err)
	}
	return s.sessionRepo.Delete(sessionID)
}

// BatchDeleteSessions 批量删除会话
func (s *chatService) BatchDeleteSessions(ctx context.Context, userID uint, sessionIDs []uint, deleteAll bool) error {
	if deleteAll {
		sessions, _, err := s.sessionRepo.ListByUser(userID, "", 1, 10000)
		if err != nil {
			return err
		}
		sessionIDs = make([]uint, 0, len(sessions))
		for _, sess := range sessions {
			sessionIDs = append(sessionIDs, sess.ID)
		}
	}
	for _, id := range sessionIDs {
		_ = s.messageRepo.DeleteBySession(id)
		_ = s.sessionRepo.Delete(id)
	}
	return nil
}

// PinSession 置顶/取消置顶
func (s *chatService) PinSession(ctx context.Context, sessionID uint, userID uint, pinned bool) error {
	session, err := s.sessionRepo.FindByID(sessionID)
	if err != nil || session == nil {
		return bizerrors.New(bizerrors.CodeResourceNotFound, "会话不存在")
	}
	if session.UserID != userID {
		return bizerrors.New(bizerrors.CodeForbidden, "无权操作")
	}
	return s.sessionRepo.UpdatePin(sessionID, pinned)
}

// StopChat 停止对话（阶段一简化实现）
func (s *chatService) StopChat(ctx context.Context, sessionID uint, userID uint, messageID string) error {
	return nil
}

// ClearSessionMessages 清空会话消息
func (s *chatService) ClearSessionMessages(ctx context.Context, sessionID uint, userID uint) error {
	session, err := s.sessionRepo.FindByID(sessionID)
	if err != nil || session == nil {
		return bizerrors.New(bizerrors.CodeResourceNotFound, "会话不存在")
	}
	if session.UserID != userID {
		return bizerrors.New(bizerrors.CodeForbidden, "无权操作")
	}
	if err := s.messageRepo.DeleteBySession(sessionID); err != nil {
		return bizerrors.NewWithErr(bizerrors.CodeInternalError, "清空消息失败", err)
	}
	session.MessageCount = 0
	session.LastMessage = ""
	return s.sessionRepo.Update(session)
}

// LoadMessages 加载历史消息
func (s *chatService) LoadMessages(ctx context.Context, sessionID uint, userID uint, limit int, beforeTime *time.Time) ([]*entity.Message, error) {
	session, err := s.sessionRepo.FindByID(sessionID)
	if err != nil || session == nil {
		return nil, bizerrors.New(bizerrors.CodeResourceNotFound, "会话不存在")
	}
	if session.UserID != userID {
		return nil, bizerrors.New(bizerrors.CodeForbidden, "无权访问")
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	return s.messageRepo.ListBySession(sessionID, limit, beforeTime)
}

// SaveAssistantMessage 保存助手回复
func (s *chatService) SaveAssistantMessage(ctx context.Context, sessionID uint, content string, references []entity.Reference, agentSteps entity.AgentSteps, isCompleted bool, agentDurationMs int64, isFallback bool) error {
	// 将 References 转换为 JSON 格式以便存储
	var refsJSON *entity.ReferenceJSONs
	if len(references) > 0 {
		refsJSON = &entity.ReferenceJSONs{}
		for _, r := range references {
			*refsJSON = append(*refsJSON, entity.ReferenceJSON{
				ChunkID:        r.ChunkID,
				Content:        r.Content,
				Score:          r.Score,
				KnowledgeID:    r.KnowledgeID,
				KnowledgeTitle: r.KnowledgeTitle,
			})
		}
	}

	// 初始化 agentSteps（避免 nil）
	if agentSteps == nil {
		agentSteps = entity.AgentSteps{}
	}

	msg := &entity.Message{
		SessionID:           sessionID,
		Role:                "assistant",
		Content:             content,
		RenderedContent:     content,
		ReferencesJSON:      refsJSON,
		KnowledgeReferences: references,
		FinishReason:        "stop",
		AgentSteps:          agentSteps,
		IsCompleted:         isCompleted,
		AgentDurationMs:     agentDurationMs,
		IsFallback:          isFallback,
	}
	if err := s.messageRepo.Create(msg); err != nil {
		return err
	}
	s.sessionRepo.IncrementMessageCount(sessionID)
	if len(content) > 200 {
		content = content[:200]
	}
	s.sessionRepo.UpdateLastMessage(sessionID, content)
	return nil
}

// buildSessionTitle 由用户首条消息生成会话标题，截断规则与 GenerateTitle 接口保持一致。
// 按 rune 计数以避免中文被截断成乱码。
func buildSessionTitle(query string) string {
	title := strings.TrimSpace(query)
	if len([]rune(title)) > 20 {
		title = string([]rune(title)[:20]) + "..."
	}
	if title == "" {
		title = "新对话"
	}
	return title
}

// GenerateSessionTitle 若会话标题仍为默认占位（"新对话"），则用首条用户消息生成标题并落库。
// 已被用户手动重命名的会话不会被覆盖。返回最终标题供调用方推送给前端。
func (s *chatService) GenerateSessionTitle(ctx context.Context, sessionID uint, userID uint, query string) (string, error) {
	session, err := s.sessionRepo.FindByID(sessionID)
	if err != nil || session == nil {
		return "", bizerrors.New(bizerrors.CodeResourceNotFound, "会话不存在")
	}
	if session.UserID != userID {
		return "", bizerrors.New(bizerrors.CodeForbidden, "无权操作")
	}
	// 仅当标题仍为默认占位时才自动生成，避免覆盖用户手动设置的标题
	if session.Title != "" && session.Title != "新对话" {
		return session.Title, nil
	}
	title := buildSessionTitle(query)
	session.Title = title
	if err := s.sessionRepo.Update(session); err != nil {
		return "", bizerrors.NewWithErr(bizerrors.CodeInternalError, "更新会话标题失败", err)
	}
	return title, nil
}
