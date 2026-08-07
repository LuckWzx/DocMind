package service

import (
	"context"
	"sync"

	"docmind/internal/model/entity"
	"docmind/pkg/logger"
)

// backfillConcurrency 存量补全的并发拉取上限（单次拉取最多 5s 超时，串行在模型较多时会拖慢启动）
const backfillConcurrency = 4

// BackfillContextWindows 存量模型 context_window 补全：
// 扫描所有聊天类（KnowledgeQA/VLLM）且未配置 context_window 的模型，
// 通过元数据接口/内置映射表解析后写回字段；单模型失败不影响整体，返回补全成功数。
// 仅在服务启动时由 app 层后台调用，对话链路不经过此方法。
func (s *modelService) BackfillContextWindows(ctx context.Context) (int, error) {
	models, err := s.modelRepo.ListAll("")
	if err != nil {
		return 0, err
	}

	// 只处理聊天类模型：embedding/rerank/asr 没有上下文窗口概念
	candidates := make([]*entity.Model, 0, len(models))
	for _, m := range models {
		if (m.Type == entity.ModelTypeKnowledgeQA || m.Type == entity.ModelTypeVLLM) && m.Parameters.ContextWindow <= 0 {
			candidates = append(candidates, m)
		}
	}
	if len(candidates) == 0 {
		return 0, nil
	}

	sem := make(chan struct{}, backfillConcurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	filled := 0
	for _, m := range candidates {
		if ctx.Err() != nil {
			break
		}
		m := m
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			if w, reason := s.resolveContextWindow(m); w > 0 {
				m.Parameters.ContextWindow = w
				if err := s.modelRepo.Update(m); err != nil {
					logger.Warnf("[ModelContextWindow] 存量模型 %s 上下文大小写回失败: %v", m.Name, err)
					return
				}
				s.clearMissingContextWindow(m.ID)
				mu.Lock()
				filled++
				mu.Unlock()
				logger.Infof("[ModelContextWindow] 存量模型 %s 上下文大小补全为 %d", m.Name, w)
			} else {
				logger.Warnf("[ModelContextWindow] 存量模型 %s 上下文大小无法确定: %s", m.Name, reason)
				s.recordMissingContextWindow(m, reason)
			}
		}()
	}
	wg.Wait()
	return filled, nil
}
