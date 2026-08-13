package service

import (
	"context"
	"sync"
	"time"

	"docmind/internal/memory"
	"docmind/internal/model/entity"
	bizerrors "docmind/pkg/errors"
	"docmind/pkg/logger"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// 短期记忆状态查询与手动压缩（前端上下文状态条数据源）。
//
// 触发机制（与对话链路一致）：token 超阈值（窗口 × 50%）或增量轮数达分档阈值，
// 任一满足即触发自动压缩；手动压缩跳过触发判断直接执行增量合并。
// 状态计算口径与 KnowledgeChat/AgentChat 的加载逻辑保持一致：
// 摘要（若有）+ 边界之后的增量消息（上限 maxIncrementalMessages）。

// memorySnapshot 一次会话短期记忆的完整快照（状态查询与手动压缩共用）
type memorySnapshot struct {
	status      *MemoryStatus
	summary     *entity.SessionSummary
	incremental []*schema.Message
	boundaryIDs []uint // 与 incremental 一一对应（写回压缩边界用）
}

// loadMemorySnapshot 加载会话短期记忆快照：校验归属 → 解析模型/窗口 → 读摘要 → 加载增量 → 计算状态。
// 未开启多轮对话时返回 nil（前端不展示状态条）。
func (s *chatService) loadMemorySnapshot(ctx context.Context, sessionID uint, userID uint) (*memorySnapshot, error) {
	session, err := s.sessionRepo.FindByID(sessionID)
	if err != nil || session == nil {
		return nil, bizerrors.New(bizerrors.CodeResourceNotFound, "会话不存在")
	}
	if session.UserID != userID {
		return nil, bizerrors.New(bizerrors.CodeForbidden, "无权访问该会话")
	}

	// 多轮对话未开启 → 无短期记忆，返回 nil
	multiTurn := false
	if session.AgentID != "" {
		if agent, aErr := s.agentSvc.ResolveForUser(userID, session.AgentID); aErr == nil && agent != nil && agent.Config.MultiTurnEnabled != nil {
			multiTurn = *agent.Config.MultiTurnEnabled
		}
	}
	if !multiTurn && session.AgentConfig != nil && session.AgentConfig.MultiTurnEnabled != nil {
		multiTurn = *session.AgentConfig.MultiTurnEnabled
	}
	if !multiTurn {
		return nil, nil
	}

	modelID := s.ResolveChatModelID(ctx, userID, session)
	maxContextTokens := memory.DefaultMaxContextTokens
	if w := s.resolveContextWindowForModel(modelID, userID); w > 0 {
		maxContextTokens = w
	}

	status := &MemoryStatus{
		ModelID:        modelID,
		ContextWindow:  maxContextTokens,
		TokenThreshold: int(float64(maxContextTokens) * memory.DefaultThresholdRatio),
		TurnsThreshold: memory.TurnsThresholdForWindow(maxContextTokens),
	}

	// 读摘要 + 加载增量（与对话链路一致：有摘要走边界，无摘要全量加载）
	var incremental []*schema.Message
	boundaryIDs := make([]uint, 0, 64)
	summaryContent := ""
	summary, sumErr := s.summaryRepo.GetBySession(sessionID)
	if sumErr != nil {
		logger.Warnf("[MemoryStatus] 读取会话摘要失败（按无摘要处理）: %v", sumErr)
		summary = nil
	}
	if summary != nil {
		summaryContent = summary.Content
		status.SummaryType = summary.SummaryType
		status.CompressedCount = summary.CompressedCount
		if msgs, listErr := s.messageRepo.ListAfterID(sessionID, summary.LastMessageID, maxIncrementalMessages); listErr == nil {
			for _, m := range msgs {
				boundaryIDs = append(boundaryIDs, m.ID)
				incremental = append(incremental, toSchemaMessage(m))
			}
		}
	} else {
		if msgs, listErr := s.messageRepo.ListBySession(sessionID, maxIncrementalMessages, nil); listErr == nil {
			for _, m := range msgs {
				boundaryIDs = append(boundaryIDs, m.ID)
				incremental = append(incremental, toSchemaMessage(m))
			}
		}
	}

	status.CurrentTokens = s.tokenEstimator.EstimateString(summaryContent) + s.tokenEstimator.EstimateMessages(incremental)
	status.CurrentTurns = memory.CountUserTurns(incremental)
	if total, tErr := s.messageRepo.CountUserTurnsBySession(sessionID); tErr == nil {
		status.TotalTurns = int(total)
	}
	status.Attention = status.CurrentTokens > status.TokenThreshold ||
		(status.TurnsThreshold > 0 && status.CurrentTurns >= status.TurnsThreshold)

	return &memorySnapshot{
		status:      status,
		summary:     summary,
		incremental: incremental,
		boundaryIDs: boundaryIDs,
	}, nil
}

// GetMemoryStatus 计算会话短期记忆状态（前端上下文状态条展示）
func (s *chatService) GetMemoryStatus(ctx context.Context, sessionID uint, userID uint) (*MemoryStatus, error) {
	snap, err := s.loadMemorySnapshot(ctx, sessionID, userID)
	if err != nil {
		return nil, err
	}
	if snap == nil {
		return nil, nil
	}
	return snap.status, nil
}

// CompressMemory 手动压缩会话短期记忆：跳过触发判断直接执行增量合并，
// 摘要写回（LastMessageID 边界推进），返回压缩后的最新状态。
// 手动压缩为强制语义：保留最近 1 轮原文（含当前轮），其余增量全部并入摘要，
// token 预算仅作防爆窗兜底——与自动压缩的"预算足够就全保留"相反。
// 同一会话的压缩互斥（busy 时返回 409），避免并发压缩重复执行/边界错乱。
func (s *chatService) CompressMemory(ctx context.Context, sessionID uint, userID uint) (*MemoryStatus, error) {
	release, ok := acquireCompressLock(sessionID)
	if !ok {
		return nil, bizerrors.New(bizerrors.CodeConflict, "该会话正在压缩中，请稍后再试")
	}
	defer release()

	snap, err := s.loadMemorySnapshot(ctx, sessionID, userID)
	if err != nil {
		return nil, err
	}
	if snap == nil {
		return nil, bizerrors.New(bizerrors.CodeBadRequest, "该会话未开启多轮对话，无需压缩")
	}
	if len(snap.incremental) <= 1 {
		// 增量不足（仅当前轮或为空），无可压缩内容
		return snap.status, nil
	}

	consolidator := memory.NewConsolidator(
		func(ctx context.Context) (model.BaseModel[*schema.Message], error) {
			return s.modelFactory.CreateChatModel(ctx, snap.status.ModelID)
		},
		s.tokenEstimator,
		snap.status.ContextWindow,
		0, // 触发比例默认 0.5
		0, // 手动压缩的保底轮数由 Forced 参数显式指定，不使用实例配置
		0,
	)
	summaryContent := ""
	if snap.summary != nil {
		summaryContent = snap.summary.Content
	}
	// 强制压缩：保留最近 1 轮原文（含当前轮），其余并入摘要
	newSummary, count, isRaw := consolidator.ConsolidateIncrementalForced(ctx, summaryContent, snap.incremental, 1)
	if count > 0 {
		summaryType := entity.SummaryTypeLLM
		if isRaw {
			summaryType = entity.SummaryTypeRaw
		}
		compressedCount := count
		if snap.summary != nil {
			compressedCount += snap.summary.CompressedCount
		}
		if err := s.summaryRepo.Upsert(&entity.SessionSummary{
			SessionID:       sessionID,
			Content:         newSummary,
			SummaryType:     summaryType,
			LastMessageID:   snap.boundaryIDs[count-1],
			CompressedCount: compressedCount,
		}); err != nil {
			logger.Warnf("[MemoryConsolidator] 手动压缩摘要写回失败: %v", err)
			return nil, bizerrors.NewWithErr(bizerrors.CodeInternalError, "摘要写回失败", err)
		}
		logger.Infof("[MemoryConsolidator] 手动压缩：%d 条消息并入摘要（降级=%v）", count, isRaw)
	}

	// 返回压缩后的最新状态（携带本次实际压缩轮数）
	status, statusErr := s.GetMemoryStatus(ctx, sessionID, userID)
	if statusErr != nil {
		return nil, statusErr
	}
	if status != nil {
		status.LastCompressedCount = count
	}
	return status, nil
}

// ============ 会话级压缩互斥锁 ============

// compressLocks 手动压缩的会话级互斥：key = sessionID，避免同一会话并发压缩
// （并发读同一边界 → 重复 LLM 调用 / 边界写回错乱）。锁对象常驻内存，
// 单把锁开销极小（几十字节），会话数量级下无需清理。
var compressLocks sync.Map // map[uint]*sessionCompressLock

type sessionCompressLock struct {
	mu       sync.Mutex
	lastUsed time.Time
}

// acquireCompressLock 尝试获取会话压缩锁；已有压缩进行中时返回 false。
func acquireCompressLock(sessionID uint) (release func(), ok bool) {
	now := time.Now()
	v, _ := compressLocks.LoadOrStore(sessionID, &sessionCompressLock{lastUsed: now})
	l, _ := v.(*sessionCompressLock)
	if !l.mu.TryLock() {
		return nil, false
	}
	l.lastUsed = now
	return func() { l.mu.Unlock() }, true
}
