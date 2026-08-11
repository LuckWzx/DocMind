package mcp

import (
	"context"
	"sync"

	"docmind/internal/model/entity"

	"github.com/mark3labs/mcp-go/client"
)

// mcpConn 单个 MCP 服务的连接条目
type mcpConn struct {
	cli client.MCPClient
}

// Manager MCP 连接管理器：按服务 ID 缓存连接，供 Agent 工具集与测试接口复用。
// 配置变更（更新/删除）时调用 Close 强制断开，下次访问自动重建。
type Manager struct {
	mu    sync.RWMutex
	conns map[uint]*mcpConn
}

// NewManager 创建 MCP 连接管理器
func NewManager() *Manager {
	return &Manager{
		conns: make(map[uint]*mcpConn),
	}
}

// GetClient 获取服务的已缓存连接，未连接则按配置创建并缓存（幂等）
func (m *Manager) GetClient(ctx context.Context, svc *entity.MCPService) (client.MCPClient, error) {
	m.mu.RLock()
	if entry, ok := m.conns[svc.ID]; ok {
		m.mu.RUnlock()
		return entry.cli, nil
	}
	m.mu.RUnlock()

	cli, err := Connect(ctx, svc)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if entry, ok := m.conns[svc.ID]; ok {
		// 并发下其他协程已建连，复用并释放本次连接
		cli.Close()
		return entry.cli, nil
	}
	m.conns[svc.ID] = &mcpConn{cli: cli}
	return cli, nil
}

// Test 测试服务连通性：临时建连（不缓存），返回工具/资源清单
func (m *Manager) Test(ctx context.Context, svc *entity.MCPService) (*TestResult, error) {
	cli, err := Connect(ctx, svc)
	if err != nil {
		return nil, err
	}
	defer cli.Close()

	headers := BuildHeaders(svc)
	tools, err := ListTools(ctx, cli, headers)
	if err != nil {
		return &TestResult{Connected: true}, err
	}
	resources, err := ListResources(ctx, cli, headers)
	if err != nil {
		// 资源列表拉取失败不阻断测试：部分 Server 未实现 resources 能力
		resources = nil
	}
	return &TestResult{
		Connected: true,
		Tools:     tools,
		Resources: resources,
	}, nil
}

// Close 断开指定服务的连接并清除缓存（更新/删除配置后调用）
func (m *Manager) Close(id uint) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if entry, ok := m.conns[id]; ok {
		entry.cli.Close()
		delete(m.conns, id)
	}
}

// CloseAll 断开全部连接（应用优雅关闭时调用）
func (m *Manager) CloseAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, entry := range m.conns {
		entry.cli.Close()
		delete(m.conns, id)
	}
}

// TestResult 连通性测试结果
type TestResult struct {
	Connected bool                        `json:"connected"`
	Tools     []entity.MCPServiceTool     `json:"tools,omitempty"`
	Resources []entity.MCPServiceResource `json:"resources,omitempty"`
}
