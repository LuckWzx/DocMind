package service

import (
	"context"
	"fmt"
	"strings"
	"time"

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
	sessionRepo     repository.SessionRepository
	messageRepo     repository.MessageRepository
	modelFactory    *ChatModelFactory
	embedderFactory *EmbedderFactory
	kbRepo          repository.KnowledgeBaseRepository
	vectorStoreRepo repository.VectorStoreRepository
	agentRepo       repository.AgentRepository
	primaryDB       *gorm.DB
	ragPipeline     *pipeline.Pipeline
	pipelineDeps    *pipeline.PipelineDeps
}

// NewChatService 创建对话服务
func NewChatService(
	sessionRepo repository.SessionRepository,
	messageRepo repository.MessageRepository,
	modelFactory *ChatModelFactory,
	embedderFactory *EmbedderFactory,
	kbRepo repository.KnowledgeBaseRepository,
	vectorStoreRepo repository.VectorStoreRepository,
	agentRepo repository.AgentRepository,
	primaryDB *gorm.DB,
) (ChatService, error) {
	// 构建 Pipeline 依赖
	pipelineDeps := &pipeline.PipelineDeps{
		EmbedderFactory: &pipelineEmbedderFactoryAdapter{factory: embedderFactory},
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
		sessionRepo:     sessionRepo,
		messageRepo:     messageRepo,
		modelFactory:    modelFactory,
		embedderFactory: embedderFactory,
		kbRepo:          kbRepo,
		vectorStoreRepo: vectorStoreRepo,
		agentRepo:       agentRepo,
		primaryDB:       primaryDB,
		ragPipeline:     ragPipeline,
		pipelineDeps:    pipelineDeps,
	}, nil
}

// pipelineEmbedderFactoryAdapter 适配 pipeline.PipelineEmbedderFactory 接口
type pipelineEmbedderFactoryAdapter struct {
	factory *EmbedderFactory
}

func (a *pipelineEmbedderFactoryAdapter) CreateEmbedder(ctx context.Context, modelID string) (pipeline.PipelineEmbedder, error) {
	return a.factory.CreatePipelineEmbedder(ctx, modelID)
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

	// 3. 加载最近对话历史（排除刚保存的 userMsg）
	const maxHistoryMessages = 20
	historyMsgs, err := s.messageRepo.ListBySession(sessionID, maxHistoryMessages+1, nil)
	if err != nil {
		historyMsgs = nil // 加载失败不影响对话
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

	// 4. 从 Agent 解析配置
	agentConfig := s.resolveAgentConfig(session, req)

	// 5. 创建 Pipeline 上下文
	pipelineCtx := &pipeline.Context{
		Query:           req.Query,
		SessionID:       sessionID,
		UserID:          userID,
		AgentConfig:     agentConfig,
		HistoryMessages: history,
		ModelRepo:       s.modelFactory.modelRepo,
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
			ChunkID:     r.ChunkID,
			Content:     r.Content,
			Score:       r.Score,
			KnowledgeID: r.KnowledgeID,
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
func (s *chatService) resolveAgentConfig(session *entity.Session, req *KnowledgeChatRequest) *pipeline.AgentConfig {
	config := &pipeline.AgentConfig{
		ModelID:            "default",
		KnowledgeBaseIDs:   req.KnowledgeBaseIDs,
		SystemPrompt:       knowledgeQASystemPrompt,
		Temperature:        defaultTemperature,
		MaxTokens:          2048,
		EnableQueryRewrite: false,
		EmbeddingTopK:      defaultTopK,
		VectorThreshold:    0.5,
		RerankTopK:         defaultTopK,
	}

	// 如果 Session 关联了 Agent，从 Agent 配置中解析
	if session.AgentID != "" {
		agent, err := s.agentRepo.FindByIDStr(session.AgentID)
		if err == nil && agent != nil {
			if agent.Config.ModelID != "" {
				config.ModelID = agent.Config.ModelID
			}
			if len(agent.Config.KnowledgeBases) > 0 {
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
		}
	}

	return config
}

// embedQuery 使用 Embedder 将查询文本转为向量，同时返回使用的模型 ID
func (s *chatService) embedQuery(ctx context.Context, kbIDs []string, query string) ([]float32, string, error) {
	if len(kbIDs) == 0 {
		return nil, "", fmt.Errorf("未指定知识库")
	}

	kbID := parseUint(kbIDs[0])
	kb, err := s.kbRepo.FindByID(kbID)
	if err != nil || kb == nil {
		return nil, "", fmt.Errorf("知识库不存在: %s", kbIDs[0])
	}

	embedderID := kb.EmbeddingModelID
	if embedderID == "" {
		embedderID = "default"
	}

	embedder, err := s.embedderFactory.CreateEmbedder(ctx, embedderID)
	if err != nil {
		return nil, "", fmt.Errorf("创建 Embedder 失败: %w", err)
	}

	vectors, err := embedder.EmbedStrings(ctx, []string{query})
	if err != nil {
		return nil, "", fmt.Errorf("Embedding 调用失败: %w", err)
	}
	if len(vectors) == 0 {
		return nil, "", fmt.Errorf("Embedding 返回空结果")
	}

	// 将 []float64 转为 []float32
	vec := vectors[0]
	result := make([]float32, len(vec))
	for i, v := range vec {
		result[i] = float32(v)
	}
	return result, embedderID, nil
}

// vectorSearch 执行向量检索
func (s *chatService) vectorSearch(ctx context.Context, kbIDs []string, queryVector []float32) ([]VectorSearchResult, error) {
	if len(kbIDs) == 0 {
		return nil, nil
	}

	// 查找默认向量存储
	stores, _, err := s.vectorStoreRepo.ListByUser(0, 1, 1)
	if err != nil || len(stores) == 0 {
		return nil, fmt.Errorf("未配置向量存储")
	}
	store := stores[0]

	// 创建向量驱动
	driver, cleanup, err := s.getVectorDriver(store)
	if err != nil {
		return nil, fmt.Errorf("创建向量驱动失败: %w", err)
	}
	defer cleanup()

	// 转换 KB IDs
	uintKBIDs := make([]uint, 0, len(kbIDs))
	for _, id := range kbIDs {
		uintKBIDs = append(uintKBIDs, parseUint(id))
	}

	results, err := driver.Search(ctx, VectorSearchParams{
		UserID:           0,
		VectorStoreID:    store.ID,
		KnowledgeBaseIDs: uintKBIDs,
		QueryVector:      queryVector,
		TopK:             defaultTopK,
		Threshold:        0.5,
	})
	if err != nil {
		return nil, fmt.Errorf("向量检索失败: %w", err)
	}

	return results, nil
}

// getVectorDriver 根据 VectorStore 创建驱动
func (s *chatService) getVectorDriver(store *entity.VectorStore) (VectorDriver, func(), error) {
	switch strings.ToLower(store.EngineType) {
	case entity.VectorStoreEnginePostgres:
		db, cleanup, err := resolvePostgresDB(s.primaryDB, store)
		if err != nil {
			return nil, func() {}, err
		}
		return newPostgresVectorDriver(db, store), cleanup, nil
	default:
		return nil, func() {}, fmt.Errorf("不支持的向量引擎: %s", store.EngineType)
	}
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

// buildPrompt 构建 RAG Prompt
func (s *chatService) buildPrompt(session *entity.Session, query string, searchResults []VectorSearchResult) []*einoschema.Message {
	var docs strings.Builder
	for i, result := range searchResults {
		docs.WriteString(fmt.Sprintf("[文档 %d] (相似度: %.2f)\n%s\n\n", i+1, result.Score, result.Content))
	}

	systemContent := knowledgeQASystemPrompt
	if docs.Len() > 0 {
		systemContent += "\n\n## 检索到的文档内容\n\n" + docs.String()
	}

	return []*einoschema.Message{
		{
			Role:    einoschema.System,
			Content: systemContent,
		},
		{
			Role:    einoschema.User,
			Content: query,
		},
	}
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
func (s *chatService) SaveAssistantMessage(ctx context.Context, sessionID uint, content string, references []entity.Reference) error {
	msg := &entity.Message{
		SessionID:           sessionID,
		Role:                "assistant",
		Content:             content,
		RenderedContent:     content,
		KnowledgeReferences: references,
		FinishReason:        "stop",
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

// parseUint 解析字符串为 uint
func parseUint(s string) uint {
	var n uint
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + uint(c-'0')
		}
	}
	return n
}
