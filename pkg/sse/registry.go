package sse

import "sync"

// Registry 活跃 SSE 连接注册表。
// 服务优雅关闭时向所有连接广播 SERVER_SHUTDOWN（retryable=true），
// 使前端收到明确的关停提示而不是莫名断流。
type Registry struct {
	mu      sync.Mutex
	writers map[*Writer]struct{}
}

// NewRegistry 创建注册表
func NewRegistry() *Registry {
	return &Registry{writers: make(map[*Writer]struct{})}
}

// Add 注册连接
func (r *Registry) Add(w *Writer) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.writers[w] = struct{}{}
}

// Remove 注销连接
func (r *Registry) Remove(w *Writer) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.writers, w)
}

// ActiveCount 活跃连接数
func (r *Registry) ActiveCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.writers)
}

// NotifyShutdown 向所有活跃连接广播 SERVER_SHUTDOWN（retryable=true）并停止其心跳，
// 返回通知的连接数。单条广播失败不影响其他连接。
func (r *Registry) NotifyShutdown() int {
	r.mu.Lock()
	conns := make([]*Writer, 0, len(r.writers))
	for w := range r.writers {
		conns = append(conns, w)
	}
	r.mu.Unlock()

	for _, w := range conns {
		w.StopHeartbeat()
		_ = w.WriteError(ErrorContract{
			Code:         ErrCodeServerShutdown,
			Retryable:    true,
			RetryAfterMs: 5000,
			Message:      "服务正在升级维护，请稍后重试",
		})
	}
	return len(conns)
}
