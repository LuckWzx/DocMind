package longterm

import (
	"context"
	"fmt"
	"strings"
	"time"

	"docmind/pkg/logger"

	"github.com/google/uuid"
)

// memoryService 长期记忆服务编排：对话结束 → 提取落图；问答前 → 检索注入
type memoryService struct {
	repo                  MemoryRepository
	extractor             *GraphExtractor
	retrieveLimit         int
	maxEpisodesPerSession int
}

// NewMemoryService 创建长期记忆服务。
// repo 或 extractor 为 nil 时返回 nil（上层按未启用处理，全链路降级跳过）。
// maxEpisodesPerSession <= 0 时使用默认上限（DefaultMaxEpisodesPerSession）。
func NewMemoryService(repo MemoryRepository, extractor *GraphExtractor, retrieveLimit int, maxEpisodesPerSession int) MemoryService {
	if repo == nil || extractor == nil {
		return nil
	}
	if retrieveLimit <= 0 {
		retrieveLimit = DefaultRetrieveLimit
	}
	if maxEpisodesPerSession <= 0 {
		maxEpisodesPerSession = DefaultMaxEpisodesPerSession
	}
	return &memoryService{repo: repo, extractor: extractor, retrieveLimit: retrieveLimit, maxEpisodesPerSession: maxEpisodesPerSession}
}

// AddEpisode 对话结束后异步提取记忆并落图。
// modelID 为当前用户会话的对话模型（工厂内部兜底），Neo4j 不可用 / LLM 提取失败
// 均只记日志返回 error，由调用方忽略，不阻断主流程。
func (s *memoryService) AddEpisode(ctx context.Context, userID, sessionID uint, modelID, query, answer string) error {
	if !s.repo.IsAvailable() {
		return fmt.Errorf("Neo4j 不可用，跳过记忆提取")
	}

	conversation := fmt.Sprintf("User: %s\nAssistant: %s", query, answer)
	result, err := s.extractor.ExtractGraph(ctx, modelID, conversation)
	if err != nil {
		return err
	}

	// 无意义过滤：摘要与实体全为空时（寒暄/纯敏感内容等），跳过落图避免空节点脏数据。
	// 仅摘要非空仍落图（实体提取失败时摘要可独立检索）。
	if strings.TrimSpace(result.Summary) == "" && len(result.Entities) == 0 {
		logger.Infof("[LongTermMemory] 提取结果无意义，跳过落图: user=%d session=%d", userID, sessionID)
		return nil
	}

	// 单会话条数上限：超出后跳过本次录入，防止图库无限膨胀。
	count, err := s.repo.CountEpisodes(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("统计会话记忆条数失败: %w", err)
	}
	if count >= s.maxEpisodesPerSession {
		logger.Infof("[LongTermMemory] 会话记忆已达上限 %d，跳过落图: user=%d session=%d", s.maxEpisodesPerSession, userID, sessionID)
		return nil
	}

	episode := &Episode{
		ID:        uuid.NewString(),
		UserID:    userID,
		SessionID: sessionID,
		Summary:   result.Summary,
		CreatedAt: time.Now(),
	}
	if err := s.repo.SaveEpisode(ctx, episode, result.Entities, result.Relationships); err != nil {
		return err
	}
	logger.Infof("[LongTermMemory] 记忆落图: user=%d session=%d entities=%d relations=%d",
		userID, sessionID, len(result.Entities), len(result.Relationships))
	return nil
}

// RetrieveMemory 问答前检索相关记忆。
// modelID 为当前用户会话的对话模型（工厂内部兜底），Neo4j 不可用或检索失败返回 nil（调用方跳过注入，不阻断对话）。
func (s *memoryService) RetrieveMemory(ctx context.Context, userID uint, modelID, query string) (*MemoryContext, error) {
	if !s.repo.IsAvailable() {
		return nil, nil
	}

	keywords, err := s.extractor.ExtractKeywords(ctx, modelID, query)
	if err != nil {
		return nil, err
	}
	if len(keywords) == 0 {
		return &MemoryContext{}, nil
	}

	episodes, err := s.repo.FindRelatedEpisodes(ctx, userID, keywords, s.retrieveLimit)
	if err != nil {
		return nil, err
	}
	if len(episodes) == 0 {
		return &MemoryContext{}, nil
	}
	logger.Infof("[LongTermMemory] 检索命中: user=%d keywords=%v episodes=%d", userID, keywords, len(episodes))
	return &MemoryContext{RelatedEpisodes: episodes}, nil
}
