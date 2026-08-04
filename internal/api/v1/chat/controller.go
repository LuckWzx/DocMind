package chat

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"docmind/internal/middleware"
	"docmind/internal/model/entity"

	"docmind/internal/service"
	"docmind/pkg/response"

	"github.com/gin-gonic/gin"
)

// Controller 会话与对话控制器
type Controller struct {
	chatService service.ChatService
}

// NewController 创建 Chat 控制器
func NewController(chatService service.ChatService) *Controller {
	return &Controller{chatService: chatService}
}

// ===== SSE 事件结构体 =====

type sseEvent struct {
	ResponseType string      `json:"response_type"`
	Content      string      `json:"content,omitempty"`    // 消息内容（前端读取此字段）
	ID           string      `json:"id,omitempty"`         // 消息 ID
	Done         bool        `json:"done,omitempty"`       // 流是否结束
	SessionID    string      `json:"session_id,omitempty"` // 会话 ID
	References   []reference `json:"references,omitempty"`
	ErrorMessage string      `json:"error_message,omitempty"`
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
	userID := middleware.GetUserID(c)
	sessionID, ok := parseUintFromPath(c, "session_id")
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	beforeTimeStr := c.Query("before_time")

	var beforeTime *time.Time
	if beforeTimeStr != "" {
		if t, err := time.Parse(time.RFC3339, beforeTimeStr); err == nil {
			beforeTime = &t
		}
	}

	messages, err := ctrl.chatService.LoadMessages(c.Request.Context(), sessionID, userID, limit, beforeTime)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, messages)
}

// ===== 核心：SSE 知识问答 =====

// KnowledgeChat SSE 流式知识问答
func (ctrl *Controller) KnowledgeChat(c *gin.Context) {
	userID := middleware.GetUserID(c)
	sessionID, ok := parseUintFromPath(c, "session_id")
	if !ok {
		return
	}

	var req service.KnowledgeChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// 设置 SSE 响应头
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Writer.Flush()

	// 调用 ChatService 获取流式响应
	stream, searchResults, err := ctrl.chatService.KnowledgeChat(c.Request.Context(), sessionID, userID, &req)
	if err != nil {
		writeSSEEvent(c.Writer, sseEvent{
			ResponseType: "error",
			Content:      err.Error(),
		})
		c.Writer.Flush()
		return
	}

	// 先发送 references 事件（即使为空也要发送，以便前端显示检索进度）
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
	writeSSEEvent(c.Writer, sseEvent{
		ResponseType: "references",
		References:   refs,
	})
	c.Writer.Flush()

	// 记录开始时间，用于计算 agent 执行时长
	agentStartTime := time.Now()

	// 流式读取 LLM 回复并推送给前端
	var fullContent string
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			writeSSEEvent(c.Writer, sseEvent{
				ResponseType: "error",
				Content:      fmt.Sprintf("流式读取失败: %v", err),
			})
			c.Writer.Flush()
			return
		}
		if chunk.Content != "" {
			fullContent += chunk.Content
			writeSSEEvent(c.Writer, sseEvent{
				ResponseType: "answer",
				Content:      chunk.Content,
			})
			c.Writer.Flush()
		}
	}

	// 计算 agent 执行时长
	agentDurationMs := time.Since(agentStartTime).Milliseconds()

	// 构建 agent_steps（RAG 管道的两个步骤：query_understand 和 knowledge_search）
	agentSteps := entity.AgentSteps{
		{
			Iteration: 1,
			Timestamp: agentStartTime,
			Duration:  0, // 第一步几乎不耗时
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

	// 判断是否为兜底回复（没有搜索结果）
	isFallback := len(searchResults) == 0

	// 保存助手消息
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

	// 首条消息后自动生成会话标题并推送，前端据此把侧栏的“新对话”更新为实际标题。
	// 仅当标题仍为默认占位时生成，避免覆盖用户手动重命名的会话。
	if req.Query != "" {
		if newTitle, err := ctrl.chatService.GenerateSessionTitle(c.Request.Context(), sessionID, userID, req.Query); err == nil && newTitle != "" && newTitle != "新对话" {
			writeSSEEvent(c.Writer, sseEvent{
				ResponseType: "session_title",
				Content:      newTitle,
				SessionID:    strconv.FormatUint(uint64(sessionID), 10),
			})
			c.Writer.Flush()
		}
	}

	// 发送完成事件
	writeSSEEvent(c.Writer, sseEvent{
		ResponseType: "complete",
		Done:         true,
		SessionID:    strconv.FormatUint(uint64(sessionID), 10),
	})
	c.Writer.Flush()
}

// AgentChat Agent 聊天桩（阶段二）
func (ctrl *Controller) AgentChat(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Writer.Flush()

	writeSSEEvent(c.Writer, sseEvent{
		ResponseType: "error",
		ErrorMessage: "Agent 模式暂未实现，将在阶段二支持",
	})
	c.Writer.Flush()
}

// ===== 辅助函数 =====

func writeSSEEvent(w http.ResponseWriter, event sseEvent) {
	data, _ := json.Marshal(event)
	fmt.Fprintf(w, "event: message\ndata: %s\n\n", string(data))
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
