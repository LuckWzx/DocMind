package chat

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// ErrUserStopped 用户主动停止生成的取消原因。
// 与客户端断连（context.Canceled）区分：主动停止时保存已生成的部分内容落库，断连不保存。
var ErrUserStopped = errors.New("user stopped generation")

// RunEntry 运行中对话任务的取消句柄
type RunEntry struct {
	cancel context.CancelCauseFunc
}

// RunRegistry 会话运行任务注册表。
// 后端在 SSE 流式回答进行中支持通过独立 stop 请求按 userID:sessionID 定位并取消任务，
// 使"停止生成"不再依赖客户端断开连接。线程安全，所有操作幂等。
type RunRegistry struct {
	mu   sync.RWMutex
	runs map[string]*RunEntry
}

// NewRunRegistry 创建运行任务注册表
func NewRunRegistry() *RunRegistry {
	return &RunRegistry{runs: make(map[string]*RunEntry)}
}

// Register 注册运行任务；同 key 已存在旧任务时先按断连语义取消旧任务
// （同一会话同一时刻仅一个流，新流启动意味着旧流被顶替，不落库）
func (r *RunRegistry) Register(key string, cancel context.CancelCauseFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if old, ok := r.runs[key]; ok && old != nil {
		old.cancel(context.Canceled)
	}
	r.runs[key] = &RunEntry{cancel: cancel}
}

// Cancel 取消指定运行任务（用户主动停止语义）；返回是否命中正在运行的任务
func (r *RunRegistry) Cancel(key string) bool {
	r.mu.RLock()
	entry, ok := r.runs[key]
	r.mu.RUnlock()
	if !ok || entry == nil {
		return false
	}
	entry.cancel(ErrUserStopped)
	return true
}

// Remove 移除运行任务（流结束时调用，幂等）
func (r *RunRegistry) Remove(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.runs, key)
}

// runKey 生成注册表 key：userID:sessionID，防止跨用户停止
func runKey(userID, sessionID uint) string {
	return fmt.Sprintf("%d:%d", userID, sessionID)
}
