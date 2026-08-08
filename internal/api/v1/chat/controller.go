package chat

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"runtime/debug"
	"strconv"
	"time"

	"docmind/internal/agent"
	"docmind/internal/middleware"
	"docmind/internal/model/entity"
	"docmind/internal/pipeline"
	"docmind/internal/service"
	"docmind/pkg/config"
	"docmind/pkg/logger"
	"docmind/pkg/response"
	"docmind/pkg/sse"

	einoschema "github.com/cloudwego/eino/schema"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Controller 会话与对话控制器
type Controller struct {
	chatService service.ChatService
	sseRegistry *sse.Registry
	redis       *redis.Client
	sseCfg      config.SSEConfig
}

// NewController 创建 Chat 控制器
func NewController(chatService service.ChatService, sseRegistry *sse.Registry, redis *redis.Client, sseCfg config.SSEConfig) *Controller {
	return &Controller{
		chatService: chatService,
		sseRegistry: sseRegistry,
		redis:       redis,
		sseCfg:      sseCfg,
	}
}

// ===== SSE 事件结构体 =====

type sseEvent struct {
	ResponseType string                      `json:"response_type"`
	Content      string                      `json:"content,omitempty"`    // 消息内容（前端读取此字段）
	ID           string                      `json:"id,omitempty"`         // 消息 ID（会话消息绑定用，非 SSE 协议事件 ID）
	Done         bool                        `json:"done,omitempty"`       // 流是否结束
	SessionID    string                      `json:"session_id,omitempty"` // 会话 ID
	References   []reference                 `json:"references,omitempty"`
	ErrorMessage string                      `json:"error_message,omitempty"`
	Ts           int64                       `json:"ts,omitempty"`           // 事件时间戳（毫秒）
	ToolCallID   string                      `json:"tool_call_id,omitempty"` // 工具调用 ID（agent_step 事件用）
	ToolName     string                      `json:"tool_name,omitempty"`    // 工具名称（agent_step 事件用）
	ToolResult   *entity.AgentStepToolResult `json:"tool_result,omitempty"`  // 工具调用结果（agent_step 事件用）
	Duration     int64                       `json:"duration,omitempty"`     // 工具调用耗时（毫秒，agent_step 事件用）
	State        string                      `json:"state,omitempty"`        // 状态机状态（thinking/searching/generating/cancelled）
	ToolArgs     string                      `json:"tool_args,omitempty"`    // 工具调用参数（JSON）
}

type reference struct {
	ChunkID        uint    `json:"chunk_id"`
	Content        string  `json:"content"`
	Score          float64 `json:"score"`
	KnowledgeID    uint    `json:"knowledge_id"`
	KnowledgeTitle string  `json:"knowledge_title"`
}

// ===== 会话 CRUD =====

// CreateSession 创建会话
func (ctrl *Controller) CreateSession(c *gin.Context) {
	userID := middleware.GetUserID(c)
	var req service.CreateSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	session, err := ctrl.chatService.CreateSession(c.Request.Context(), userID, &req)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, session)
}

// ListSessions 获取会话列表
func (ctrl *Controller) ListSessions(c *gin.Context) {
	userID := middleware.GetUserID(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	source := c.Query("source")

	sessions, total, err := ctrl.chatService.ListSessions(c.Request.Context(), userID, source, page, pageSize)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, response.NewPageResponse(sessions, total, page, pageSize))
}

// GetSession 获取单个会话
func (ctrl *Controller) GetSession(c *gin.Context) {
	userID := middleware.GetUserID(c)
	sessionID, ok := parseSessionID(c)
	if !ok {
		return
	}
	session, err := ctrl.chatService.GetSession(c.Request.Context(), sessionID, userID)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, session)
}

// UpdateSession 更新会话
func (ctrl *Controller) UpdateSession(c *gin.Context) {
	userID := middleware.GetUserID(c)
	sessionID, ok := parseSessionID(c)
	if !ok {
		return
	}
	var req service.UpdateSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := ctrl.chatService.UpdateSession(c.Request.Context(), sessionID, userID, &req); err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, nil)
}

// DeleteSession 删除会话
func (ctrl *Controller) DeleteSession(c *gin.Context) {
	userID := middleware.GetUserID(c)
	sessionID, ok := parseSessionID(c)
	if !ok {
		return
	}
	if err := ctrl.chatService.DeleteSession(c.Request.Context(), sessionID, userID); err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, nil)
}

// BatchDeleteSessions 批量删除会话
func (ctrl *Controller) BatchDeleteSessions(c *gin.Context) {
	userID := middleware.GetUserID(c)
	var req struct {
		IDs       []uint `json:"ids"`
		DeleteAll bool   `json:"delete_all"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := ctrl.chatService.BatchDeleteSessions(c.Request.Context(), userID, req.IDs, req.DeleteAll); err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, nil)
}

// PinSession 置顶会话
func (ctrl *Controller) PinSession(c *gin.Context) {
	userID := middleware.GetUserID(c)
	sessionID, ok := parseSessionID(c)
	if !ok {
		return
	}
	if err := ctrl.chatService.PinSession(c.Request.Context(), sessionID, userID, true); err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, nil)
}

// UnpinSession 取消置顶
func (ctrl *Controller) UnpinSession(c *gin.Context) {
	userID := middleware.GetUserID(c)
	sessionID, ok := parseSessionID(c)
	if !ok {
		return
	}
	if err := ctrl.chatService.PinSession(c.Request.Context(), sessionID, userID, false); err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, nil)
}

// StopSession 停止会话
func (ctrl *Controller) StopSession(c *gin.Context) {
	userID := middleware.GetUserID(c)
	sessionID, ok := parseSessionID(c)
	if !ok {
		return
	}
	var req struct {
		MessageID string `json:"message_id"`
	}
	_ = c.ShouldBindJSON(&req)
	if err := ctrl.chatService.StopChat(c.Request.Context(), sessionID, userID, req.MessageID); err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, nil)
}

// ClearSessionMessages 清空消息
func (ctrl *Controller) ClearSessionMessages(c *gin.Context) {
	userID := middleware.GetUserID(c)
	sessionID, ok := parseSessionID(c)
	if !ok {
		return
	}
	if err := ctrl.chatService.ClearSessionMessages(c.Request.Context(), sessionID, userID); err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, nil)
}

// GenerateTitle 自动生成标题（阶段一简化：用 query 前 20 字）
func (ctrl *Controller) GenerateTitle(c *gin.Context) {
	userID := middleware.GetUserID(c)
	sessionID, ok := parseSessionID(c)
	if !ok {
		return
	}
	var req struct {
		Query string `json:"query"`
	}
	_ = c.ShouldBindJSON(&req)

	title := req.Query
	if len(title) > 20 {
		title = title[:20] + "..."
	}
	if title == "" {
		title = "新对话"
	}

	updateReq := &service.UpdateSessionRequest{Title: &title}
	if err := ctrl.chatService.UpdateSession(c.Request.Context(), sessionID, userID, updateReq); err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, gin.H{"title": title})
}

// ===== 消息 =====

// LoadMessages 加载消息历史
func (ctrl *Controller) LoadMessages(c *gin.Context) {
	// 防止 panic 导致500错误
	defer func() {
		if r := recover(); r != nil {
			logger.Error("LoadMessages panic recovered",
				zap.Any("error", r),
				zap.String("stack", string(debug.Stack())),
				zap.String("path", c.Request.URL.Path),
			)
			response.InternalError(c, "加载消息失败")
		}
	}()

	userID := middleware.GetUserID(c)
	sessionID, ok := parseUintFromPath(c, "session_id")
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	beforeTimeStr := c.Query("before_time")

	var beforeTime *time.Time
	if beforeTimeStr != "" {
		if t, err := time.Parse(time.RFC3339, beforeTimeStr); err == nil {
			beforeTime = &t
		}
	}

	logger.Info("LoadMessages request",
		zap.Uint("session_id", sessionID),
		zap.Uint("user_id", userID),
		zap.Int("limit", limit),
	)

	messages, err := ctrl.chatService.LoadMessages(c.Request.Context(), sessionID, userID, limit, beforeTime)
	if err != nil {
		logger.Error("LoadMessages service error",
			zap.Error(err),
			zap.Uint("session_id", sessionID),
			zap.Uint("user_id", userID),
		)
		response.BizError(c, err)
		return
	}
	if messages == nil {
		messages = []*entity.Message{}
	}
	response.Success(c, messages)
}

// ===== 核心：SSE 知识问答 =====

// KnowledgeChat SSE 流式知识问答（第一批优化：事件协议规范化 + 执行护栏 + 生命周期日志）
func (ctrl *Controller) KnowledgeChat(c *gin.Context) {
	// 1. 请求体大小限制（护栏：防止超大请求拖垮服务）
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, ctrl.sseCfg.MaxBodyBytes)

	userID := middleware.GetUserID(c)
	sessionID, ok := parseUintFromPath(c, "session_id")
	if !ok {
		return
	}

	var req service.KnowledgeChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{
				"success": false,
				"code":    "PAYLOAD_TOO_LARGE",
				"message": "请求体过大",
			})
			return
		}
		response.BadRequest(c, err.Error())
		return
	}

	// 2. 请求 ID：贯穿日志与事件 ID（前端已带 X-Request-ID，缺失时后端生成）
	requestID := c.GetHeader("X-Request-ID")
	if requestID == "" {
		requestID = genRequestID()
	}

	// 3. 设置 SSE 响应头
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Writer.Flush()

	// 4. 协议 Writer：事件 ID 自增 + 心跳；注册到全局表（优雅关闭广播用）
	sw := sse.NewWriter(c.Writer, requestID)
	sw.StartHeartbeat(c.Request.Context(), ctrl.sseCfg.HeartbeatInterval)
	defer sw.StopHeartbeat()
	ctrl.sseRegistry.Add(sw)
	defer ctrl.sseRegistry.Remove(sw)

	// 5. 生命周期统计与日志
	stats := newSSEStats(requestID, userID, sessionID)
	stats.markOpen()
	defer stats.close("done") // 正常路径在 complete 后显式 close；closed 幂等保证异常路径不重复

	// 6. 总执行超时（覆盖 pre-work + 整个流）
	ctx, cancel := context.WithTimeout(c.Request.Context(), ctrl.sseCfg.TotalTimeout)
	defer cancel()

	// 7. pump goroutine：service 调用 + 流式读取（首 token 超时在 pump 内保护），
	//    使 handler 可以对 pre-work 超时、客户端断开做出响应
	out := make(chan streamItem, 64)

	// 步骤回调：实时发送 SSE 事件，并收集步骤信息用于构建 agent_steps
	var stepInfos []pipeline.StepInfo
	stepCallback := func(step pipeline.StepInfo) {
		// 收集步骤信息
		stepInfos = append(stepInfos, step)

		// 构建工具调用结果
		var toolResult *entity.AgentStepToolResult
		if step.Data != nil {
			toolResult = &entity.AgentStepToolResult{
				Success: step.Success,
				Data:    step.Data,
			}
		}

		// 发送 agent_step 事件
		sw.WriteMessage(sseEvent{
			ResponseType: SSEEventAgentStep,
			ToolCallID:   step.ToolCallID,
			ToolName:     step.StepName,
			ToolResult:   toolResult,
			Duration:     step.Duration,
			Ts:           time.Now().UnixMilli(),
		})
	}

	go pumpKnowledgeChat(ctx, ctrl.chatService, sessionID, userID, &req, ctrl.sseCfg.FirstTokenTimeout, out, stepCallback)

	// 8. pre-work 窗口：等待 references 事件（检索完成）；超时或断连即终止
	var searchResults []service.VectorSearchResult
	select {
	case item := <-out:
		if item.kind == "error" {
			sw.WriteError(contractForError(item.err, item.timeout))
			stats.close("error")
			return
		}
		if item.kind == "refs" {
			searchResults = item.refs
			stats.markFirstToken()
			// 先发送 references 事件（即使为空也要发送，以便前端显示检索进度）
			sw.WriteMessage(sseEvent{
				ResponseType: SSEEventReferences,
				References:   buildRefs(item.refs),
				Ts:           time.Now().UnixMilli(),
			})
		}
	case <-time.After(ctrl.sseCfg.FirstTokenTimeout):
		cancel() // 终止 pump 的 pre-work
		sw.WriteError(sse.ErrorContract{
			Code:         sse.ErrCodeFirstTokenTimeout,
			Retryable:    true,
			RetryAfterMs: 3000,
			Message:      "处理超时（首 token 超时），请重试",
		})
		stats.close("first_token_timeout")
		return
	case <-c.Request.Context().Done():
		stats.close("abort") // 客户端断开
		return
	}

	// 记录开始时间，用于计算 agent 执行时长
	agentStartTime := time.Now()

	// 9. 流式读取循环：逐 chunk 推送 answer 增量
	var fullContent string
	for item := range out {
		switch item.kind {
		case "answer":
			fullContent += item.content
			stats.markEvent()
			sw.WriteMessage(sseEvent{
				ResponseType: SSEEventAnswer,
				Content:      item.content,
				Ts:           time.Now().UnixMilli(),
			})
		case "error":
			if item.timeout {
				sw.WriteError(sse.ErrorContract{
					Code:         sse.ErrCodeFirstTokenTimeout,
					Retryable:    true,
					RetryAfterMs: 3000,
					Message:      item.err.Error(),
				})
			} else if ctx.Err() == context.DeadlineExceeded {
				sw.WriteError(sse.ErrorContract{
					Code:         sse.ErrCodeLLMTimeout,
					Retryable:    true,
					RetryAfterMs: 3000,
					Message:      "执行超时，请重试",
				})
			} else {
				sw.WriteError(sse.ErrorContract{
					Code:         sse.ErrCodeStreamError,
					Retryable:    true,
					RetryAfterMs: 3000,
					Message:      fmt.Sprintf("流式读取失败: %v", item.err),
				})
			}
			stats.close("error")
			return
		case "eof":
			goto streamDone
		}
	}
streamDone:

	// 10. 计算 agent 执行时长
	agentDurationMs := time.Since(agentStartTime).Milliseconds()

	// 11. 构建 agent_steps（使用步骤回调收集的实际执行信息）
	var agentSteps entity.AgentSteps
	if len(stepInfos) > 0 {
		// 使用回调收集的步骤信息
		for i, step := range stepInfos {
			agentSteps = append(agentSteps, entity.AgentStep{
				Iteration: i + 1,
				Timestamp: step.StartTime,
				Duration:  step.Duration,
				ToolCalls: []entity.AgentStepToolCall{{
					ID:   step.ToolCallID,
					Name: step.StepName,
					Result: &entity.AgentStepToolResult{
						Success: step.Success,
						Data:    step.Data,
					},
				}},
			})
		}
	} else {
		// 兜底：如果没有步骤回调（理论上不会发生）
		agentSteps = entity.AgentSteps{
			{
				Iteration: 1,
				Timestamp: agentStartTime,
				Duration:  0,
				ToolCalls: []entity.AgentStepToolCall{{
					ID:   "rag-history-query-understand",
					Name: "query_understand",
				}},
			},
			{
				Iteration: 2,
				Timestamp: agentStartTime,
				Duration:  agentDurationMs,
				ToolCalls: []entity.AgentStepToolCall{{
					ID:   "rag-history-knowledge-search",
					Name: "knowledge_search",
					Result: &entity.AgentStepToolResult{
						Success: true,
						Data: map[string]interface{}{
							"count": len(searchResults),
						},
					},
				}},
			},
		}
	}

	// 12. 判断是否为兜底回复（没有搜索结果）
	isFallback := len(searchResults) == 0

	// 13. 保存助手消息
	if fullContent != "" {
		var references []entity.Reference
		for _, r := range searchResults {
			references = append(references, entity.Reference{
				ChunkID:        r.ChunkID,
				Content:        r.Content,
				Score:          r.Score,
				KnowledgeID:    r.KnowledgeID,
				KnowledgeTitle: r.KnowledgeTitle,
			})
		}
		_ = ctrl.chatService.SaveAssistantMessage(
			c.Request.Context(), sessionID, fullContent, references,
			agentSteps, true, agentDurationMs, isFallback,
		)
	}

	// 14. 首条消息后自动生成会话标题并推送，前端据此把侧栏的“新对话”更新为实际标题。
	//     仅当标题仍为默认占位时生成，避免覆盖用户手动重命名的会话。
	if req.Query != "" {
		if newTitle, err := ctrl.chatService.GenerateSessionTitle(c.Request.Context(), sessionID, userID, req.Query); err == nil && newTitle != "" && newTitle != "新对话" {
			sw.WriteMessage(sseEvent{
				ResponseType: SSEEventSessionTitle,
				Content:      newTitle,
				SessionID:    strconv.FormatUint(uint64(sessionID), 10),
				Ts:           time.Now().UnixMilli(),
			})
		}
	}

	// 15. 发送完成事件
	sw.WriteMessage(sseEvent{
		ResponseType: SSEEventComplete,
		Done:         true,
		SessionID:    strconv.FormatUint(uint64(sessionID), 10),
		Ts:           time.Now().UnixMilli(),
	})
	stats.close("done")
}

// AgentChat 智能推理对话（AgentMode=smart-reasoning，规划 3.2.7）
// 消费引擎统一事件流并映射为 SSE 事件：state / agent_step / answer / error / complete
// 复用 KnowledgeChat 的 SSE 连接护栏：body 限制、心跳、注册表、总超时、首事件超时、生命周期统计
func (ctrl *Controller) AgentChat(c *gin.Context) {
	// 1. 请求体大小限制（护栏：防止超大请求拖垮服务，与 KnowledgeChat 一致）
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, ctrl.sseCfg.MaxBodyBytes)

	userID := middleware.GetUserID(c)
	sessionID, ok := parseUintFromPath(c, "session_id")
	if !ok {
		return
	}

	var req service.AgentChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{
				"success": false,
				"code":    "PAYLOAD_TOO_LARGE",
				"message": "请求体过大",
			})
			return
		}
		response.BadRequest(c, err.Error())
		return
	}

	// 2. 请求 ID：贯穿日志与事件 ID（前端已带 X-Request-ID，缺失时后端生成）
	requestID := c.GetHeader("X-Request-ID")
	if requestID == "" {
		requestID = genRequestID()
	}

	// 3. 设置 SSE 响应头
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Writer.Flush()

	// 4. 协议 Writer：事件 ID 自增 + 心跳；注册到全局表（优雅关闭广播用）
	sw := sse.NewWriter(c.Writer, requestID)
	sw.StartHeartbeat(c.Request.Context(), ctrl.sseCfg.HeartbeatInterval)
	defer sw.StopHeartbeat()
	ctrl.sseRegistry.Add(sw)
	defer ctrl.sseRegistry.Remove(sw)

	// 5. 生命周期统计与日志
	stats := newSSEStats(requestID, userID, sessionID)
	stats.markOpen()
	defer stats.close("done") // 正常路径在 complete 后显式 close；closed 幂等保证异常路径不重复

	// 6. 总执行超时（覆盖整个 Agent 执行 + 事件消费）
	ctx, cancel := context.WithTimeout(c.Request.Context(), ctrl.sseCfg.TotalTimeout)
	defer cancel()

	// 7. 调用 ChatService 获取引擎事件流
	resp, err := ctrl.chatService.AgentChat(ctx, sessionID, userID, &req)
	if err != nil {
		sw.WriteError(sse.ErrorContract{
			Code:      sse.ErrCodeInternal,
			Retryable: false,
			Message:   err.Error(),
		})
		stats.close("error")
		return
	}

	// 记录开始时间，用于计算 agent 执行时长
	agentStartTime := time.Now()

	// 8. 消费引擎事件流并映射 SSE 事件（规划 3.2.7 ②：一个 AgentEvent ≠ 一条 SSE，
	// 引擎展开层已处理，此处按统一事件逐条转发）
	var fullContent string

	// 9. 首事件保护：第一个事件必须在窗口内到达（引擎首个事件通常为 thinking 状态）
	first := make(chan agentFirstResult, 1)
	go func() {
		ev, ok := resp.Stream.Next()
		first <- agentFirstResult{ev: ev, ok: ok}
	}()
	select {
	case r := <-first:
		if r.ok {
			emitAgentEvent(sw, r.ev, &fullContent)
			stats.markFirstToken()
			stats.markEvent()
		}
	case <-time.After(ctrl.sseCfg.FirstTokenTimeout):
		cancel() // 终止引擎执行
		sw.WriteError(sse.ErrorContract{
			Code:         sse.ErrCodeFirstTokenTimeout,
			Retryable:    true,
			RetryAfterMs: 3000,
			Message:      "处理超时（首事件超时），请重试",
		})
		stats.close("first_token_timeout")
		return
	case <-ctx.Done():
		stats.close("abort")
		return
	}

	// 10. 后续事件循环（ctx 取消即终止）
	for {
		select {
		case <-ctx.Done():
			stats.close("abort")
			return
		default:
		}
		ev, ok := resp.Stream.Next()
		if !ok {
			break
		}
		emitAgentEvent(sw, ev, &fullContent)
		stats.markEvent()
	}

	// 计算 agent 执行时长
	agentDurationMs := time.Since(agentStartTime).Milliseconds()

	// 引用溯源（kb_search 工具收集，规划 3.2.6 ⑥）+ 步骤记录（事件流自动生成）
	references := resp.Collector.All()
	agentSteps := resp.Stream.Steps()
	refs := make([]reference, 0, len(references))
	for _, r := range references {
		refs = append(refs, reference{
			ChunkID:        r.ChunkID,
			Content:        r.Content,
			Score:          r.Score,
			KnowledgeID:    r.KnowledgeID,
			KnowledgeTitle: r.KnowledgeTitle,
		})
	}
	// 引用事件（Agent 模式检索在循环中，引用可能迟到；此处流结束后统一推送，
	// agent_step 已携带每轮工具结果，前端可选任一路径）
	if len(refs) > 0 {
		sw.WriteMessage(sseEvent{ResponseType: SSEEventReferences, References: refs})
	}

	// 保存助手消息（含引用 / agent_steps / 耗时 / 兜底标记）
	if fullContent != "" {
		_ = ctrl.chatService.SaveAssistantMessage(
			c.Request.Context(), sessionID, fullContent, references,
			agentSteps, true, agentDurationMs, len(references) == 0,
		)
	}

	// 首条消息后自动生成会话标题并推送
	if req.Query != "" {
		if newTitle, err := ctrl.chatService.GenerateSessionTitle(c.Request.Context(), sessionID, userID, req.Query); err == nil && newTitle != "" && newTitle != "新对话" {
			sw.WriteMessage(sseEvent{
				ResponseType: SSEEventSessionTitle,
				Content:      newTitle,
				SessionID:    strconv.FormatUint(uint64(sessionID), 10),
			})
		}
	}

	// 发送完成事件
	sw.WriteMessage(sseEvent{
		ResponseType: SSEEventComplete,
		Done:         true,
		SessionID:    strconv.FormatUint(uint64(sessionID), 10),
		Ts:           time.Now().UnixMilli(),
	})
	stats.close("done")
}

// agentFirstResult 首事件保护通道载荷（首事件必须超时窗口内到达，否则报超时）
type agentFirstResult struct {
	ev *agent.OutputEvent
	ok bool
}

// emitAgentEvent 将引擎统一事件映射为 SSE 事件并写入协议 Writer（writeFrame 自带 Flush）
func emitAgentEvent(sw *sse.Writer, ev *agent.OutputEvent, fullContent *string) {
	switch ev.Type {
	case agent.EventState:
		_ = sw.WriteMessage(sseEvent{ResponseType: SSEEventState, State: ev.State})
	case agent.EventStep:
		var toolResult *entity.AgentStepToolResult
		if ev.ToolResult != "" {
			toolResult = &entity.AgentStepToolResult{Success: true, Output: ev.ToolResult}
		}
		_ = sw.WriteMessage(sseEvent{
			ResponseType: SSEEventAgentStep,
			ToolName:     ev.ToolName,
			ToolArgs:     ev.ToolArgs,
			ToolResult:   toolResult,
		})
	case agent.EventAnswer:
		*fullContent += ev.Content
		_ = sw.WriteMessage(sseEvent{ResponseType: SSEEventAnswer, Content: ev.Content})
	case agent.EventError:
		_ = sw.WriteMessage(sseEvent{ResponseType: SSEEventError, Content: ev.Content})
	}
}

// ===== 辅助函数 =====

func writeSSEEvent(w http.ResponseWriter, event sseEvent) {
	data, _ := json.Marshal(event)
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", sseProtocolEvent, string(data))
}

// ===== SSE 第一批优化辅助 =====

// streamItem pump goroutine 与 handler 主循环间的通道消息
// kind: refs | answer | error | eof
type streamItem struct {
	kind    string // refs | answer | error | eof
	refs    []service.VectorSearchResult
	content string
	err     error
	timeout bool // error 是否为首 token 超时
}

// recvResult 首 chunk 读取结果（首 token 超时保护用）
type recvResult struct {
	chunk *einoschema.Message
	err   error
}

// pumpKnowledgeChat 在子 goroutine 中执行：service 调用 + 流式读取，结果经通道回传。
// 首 token 超时在 pump 内保护（流建立后第一个 chunk 必须在窗口内到达）；
// 总执行超时由外层 ctx 兜底（取消后 LLM 生成停止、流关闭、Recv 返回）。
func pumpKnowledgeChat(ctx context.Context, svc service.ChatService, sessionID, userID uint, req *service.KnowledgeChatRequest, firstTokenTimeout time.Duration, out chan<- streamItem, stepCallback pipeline.StepCallback) {
	defer close(out)
	stream, searchResults, err := svc.KnowledgeChat(ctx, sessionID, userID, req, stepCallback)
	if err != nil {
		out <- streamItem{kind: "error", err: err}
		return
	}
	out <- streamItem{kind: "refs", refs: searchResults}

	// 首 token 保护：第一个 chunk 必须在窗口内到达，否则报超时
	first := make(chan recvResult, 1)
	go func() {
		chunk, recvErr := stream.Recv()
		first <- recvResult{chunk: chunk, err: recvErr}
	}()
	select {
	case r := <-first:
		if !emitChunk(out, r.chunk, r.err) {
			return
		}
	case <-time.After(firstTokenTimeout):
		out <- streamItem{kind: "error", err: errors.New("LLM 首 token 超时"), timeout: true}
		return
	}

	// 后续流式读取
	for {
		chunk, recvErr := stream.Recv()
		if !emitChunk(out, chunk, recvErr) {
			return
		}
	}
}

// emitChunk 将单个 chunk 转为通道消息；返回 false 表示流已结束
func emitChunk(out chan<- streamItem, chunk *einoschema.Message, err error) bool {
	if err == io.EOF {
		out <- streamItem{kind: "eof"}
		return false
	}
	if err != nil {
		out <- streamItem{kind: "error", err: err}
		return false
	}
	if chunk != nil && chunk.Content != "" {
		out <- streamItem{kind: "answer", content: chunk.Content}
	}
	return true
}

// contractForError pre-work 阶段错误 → 错误契约（非 LLM 问题默认不可重试）
func contractForError(err error, timeout bool) sse.ErrorContract {
	if timeout {
		return sse.ErrorContract{Code: sse.ErrCodeFirstTokenTimeout, Retryable: true, RetryAfterMs: 3000, Message: err.Error()}
	}
	return sse.ErrorContract{Code: sse.ErrCodeInternal, Retryable: false, Message: err.Error()}
}

// buildRefs 检索结果 → SSE references 事件载荷
func buildRefs(searchResults []service.VectorSearchResult) []reference {
	refs := make([]reference, 0, len(searchResults))
	for _, r := range searchResults {
		refs = append(refs, reference{
			ChunkID:        r.ChunkID,
			Content:        r.Content,
			Score:          r.Score,
			KnowledgeID:    r.KnowledgeID,
			KnowledgeTitle: r.KnowledgeTitle,
		})
	}
	return refs
}

// genRequestID 生成请求 ID（无 X-Request-ID 时兜底，用于事件 ID 前缀与日志贯穿）
func genRequestID() string {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("req-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// sseStats SSE 连接生命周期统计（sse_open / sse_close 日志）
type sseStats struct {
	requestID  string
	userID     uint
	sessionID  uint
	openAt     time.Time
	firstToken time.Time
	events     int64
	closed     bool
}

func newSSEStats(requestID string, userID, sessionID uint) *sseStats {
	return &sseStats{requestID: requestID, userID: userID, sessionID: sessionID, openAt: time.Now()}
}

// markOpen sse_open 日志
func (s *sseStats) markOpen() {
	logger.Info("sse_open",
		zap.String("request_id", s.requestID),
		zap.Uint("user_id", s.userID),
		zap.Uint("session_id", s.sessionID),
	)
}

// markFirstToken 记录首事件（references 到达）时刻
func (s *sseStats) markFirstToken() {
	if s.firstToken.IsZero() {
		s.firstToken = time.Now()
	}
}

// markEvent 业务事件计数 + debug 级日志（防刷屏）
func (s *sseStats) markEvent() {
	s.events++
	logger.Debug("sse_event",
		zap.String("request_id", s.requestID),
		zap.Int64("seq", s.events),
	)
}

// close sse_close 汇总日志（closed 幂等：defer 与显式调用并存时只记一次）
func (s *sseStats) close(reason string) {
	if s.closed {
		return
	}
	s.closed = true
	var firstTokenMs int64
	if !s.firstToken.IsZero() {
		firstTokenMs = s.firstToken.Sub(s.openAt).Milliseconds()
	}
	logger.Info("sse_close",
		zap.String("request_id", s.requestID),
		zap.Uint("user_id", s.userID),
		zap.Uint("session_id", s.sessionID),
		zap.String("reason", reason),
		zap.Int64("duration_ms", time.Since(s.openAt).Milliseconds()),
		zap.Int64("first_token_ms", firstTokenMs),
		zap.Int64("events", s.events),
	)
}

func parseSessionID(c *gin.Context) (uint, bool) {
	id, ok := parseUintFromPath(c, "id")
	return id, ok
}

func parseUintFromPath(c *gin.Context, param string) (uint, bool) {
	idStr := c.Param(param)
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的 ID")
		return 0, false
	}
	return uint(id), true
}
