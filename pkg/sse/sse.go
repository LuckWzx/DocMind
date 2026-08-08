package sse

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// 协议层事件名（SSE 的 event: 字段）。
// 业务数据统一走 message，由 payload 内的 response_type 区分语义（与前端契约一致）；
// ping 为协议层心跳，payload 无 response_type，前端按此过滤。
const (
	ProtocolEventMessage = "message"
	ProtocolEventPing    = "ping"
)

// 错误契约错误码（与前端约定的错误分类）
const (
	ErrCodeLLMTimeout        = "LLM_TIMEOUT"         // 总执行超时
	ErrCodeFirstTokenTimeout = "FIRST_TOKEN_TIMEOUT" // 首 token 超时
	ErrCodeStreamError       = "STREAM_ERROR"        // LLM 流中断/读取失败
	ErrCodeServerShutdown    = "SERVER_SHUTDOWN"     // 服务关停（可重试）
	ErrCodeDuplicate         = "DUPLICATE_REQUEST"   // 重复请求（幂等命中）
	ErrCodeInternal          = "INTERNAL"            // 其他内部错误
)

// ErrorContract 错误契约：错误事件的标准三段式字段，
// 前端据此决定自动重试（retryable=true）或提示用户
type ErrorContract struct {
	Code         string `json:"code"`
	Retryable    bool   `json:"retryable"`
	RetryAfterMs int64  `json:"retry_after_ms,omitempty"`
	Message      string `json:"message,omitempty"`
}

// Writer SSE 写入器：事件 ID 自增（requestID:seq）、心跳、并发安全。
//
// 事件 ID 只写 SSE 协议层 id: 行（供 Last-Event-ID 断点续传），
// 不进入 JSON payload——payload 内的 id 字段是消息 ID 语义，前端用作消息绑定。
type Writer struct {
	w         http.ResponseWriter
	flusher   http.Flusher
	requestID string
	seq       int64

	mu     sync.Mutex
	hbStop chan struct{}
}

// NewWriter 创建 SSE 写入器
func NewWriter(w http.ResponseWriter, requestID string) *Writer {
	flusher, _ := w.(http.Flusher)
	return &Writer{w: w, flusher: flusher, requestID: requestID}
}

// RequestID 请求 ID
func (sw *Writer) RequestID() string { return sw.requestID }

// WriteMessage 发送业务事件（event: message），自动生成 id: requestID:seq
func (sw *Writer) WriteMessage(payload any) error {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	sw.seq++
	return sw.writeFrame(ProtocolEventMessage, fmt.Sprintf("%s:%d", sw.requestID, sw.seq), payload)
}

// WritePing 发送心跳事件（规避云 LB / NAT 空闲断连）
func (sw *Writer) WritePing() error {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	return sw.writeFrame(ProtocolEventPing, "", map[string]int64{"ts": time.Now().UnixMilli()})
}

// WriteError 发送错误事件：走 response_type=error 业务通道（前端现有错误分支兼容），
// 附带错误契约字段（error_code/retryable/retry_after_ms）
func (sw *Writer) WriteError(ec ErrorContract) error {
	payload := map[string]any{
		"response_type": "error",
		"content":       ec.Message,
		"error_code":    ec.Code,
		"retryable":     ec.Retryable,
	}
	if ec.RetryAfterMs > 0 {
		payload["retry_after_ms"] = ec.RetryAfterMs
	}
	return sw.WriteMessage(payload)
}

// StartHeartbeat 启动心跳 goroutine（幂等，重复调用不启动多个；
// 写失败或 ctx 取消时自动退出）
func (sw *Writer) StartHeartbeat(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		return
	}
	sw.mu.Lock()
	if sw.hbStop != nil {
		sw.mu.Unlock()
		return
	}
	sw.hbStop = make(chan struct{})
	stop := sw.hbStop
	sw.mu.Unlock()

	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				if err := sw.WritePing(); err != nil {
					return // 写失败（客户端断开等），停止心跳
				}
			case <-stop:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
}

// StopHeartbeat 停止心跳 goroutine
func (sw *Writer) StopHeartbeat() {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	if sw.hbStop != nil {
		close(sw.hbStop)
		sw.hbStop = nil
	}
}

// writeFrame 写一帧 SSE（须在持锁时调用，保证心跳与业务写不交错）
func (sw *Writer) writeFrame(eventName, id string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	var sb strings.Builder
	if id != "" {
		sb.WriteString("id: ")
		sb.WriteString(id)
		sb.WriteString("\n")
	}
	sb.WriteString("event: ")
	sb.WriteString(eventName)
	sb.WriteString("\ndata: ")
	sb.Write(data)
	sb.WriteString("\n\n")
	if _, err := sw.w.Write([]byte(sb.String())); err != nil {
		return err
	}
	if sw.flusher != nil {
		sw.flusher.Flush()
	}
	return nil
}
