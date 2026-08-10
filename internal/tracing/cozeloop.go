package tracing

import (
	"context"
	"fmt"

	ccb "github.com/cloudwego/eino-ext/callbacks/cozeloop"
	"github.com/cloudwego/eino/callbacks"
	"github.com/coze-dev/cozeloop-go"
	"go.uber.org/zap"

	"docmind/pkg/config"
	"docmind/pkg/logger"
)

// CozeLoopTracer CozeLoop 链路追踪实例
// 持有底层 client，供服务优雅关闭时冲刷并关闭上报连接
type CozeLoopTracer struct {
	client cozeloop.Client
}

// InitCozeLoop 初始化 CozeLoop 链路追踪（可选能力，配置不完整时静默跳过，不阻塞服务启动）
// 挂载全局 callbacks handler 后，进程内所有 Eino 组件（Agent 引擎 / RAG 管道 /
// ChatModel / Embedder / Reranker）调用自动上报 Trace。
// 注意：callbacks.AppendGlobalHandlers 非线程安全，必须在任何 Eino 组件执行前调用一次。
func InitCozeLoop(cfg *config.CozeLoopConfig) (*CozeLoopTracer, error) {
	if cfg == nil || !cfg.Enabled {
		return nil, nil
	}
	if cfg.WorkspaceID == "" || cfg.APIToken == "" {
		logger.Warn("CozeLoop 链路追踪配置不完整（workspace_id / api_token 缺失），已跳过")
		return nil, nil
	}

	opts := []cozeloop.Option{
		cozeloop.WithWorkspaceID(cfg.WorkspaceID),
		cozeloop.WithAPIToken(cfg.APIToken),
	}
	if cfg.APIBaseURL != "" {
		opts = append(opts, cozeloop.WithAPIBaseURL(cfg.APIBaseURL))
	}

	client, err := cozeloop.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("创建 CozeLoop client 失败: %w", err)
	}

	// 全局 handler：在服务 init 时调用一次
	callbacks.AppendGlobalHandlers(ccb.NewLoopHandler(client))

	baseURL := cfg.APIBaseURL
	if baseURL == "" {
		baseURL = "https://api.coze.cn" // SDK 默认国内版
	}
	logger.Info("CozeLoop 链路追踪已启用",
		zap.String("workspace_id", cfg.WorkspaceID),
		zap.String("api_base_url", baseURL),
	)
	return &CozeLoopTracer{client: client}, nil
}

// Close 关闭 CozeLoop 上报连接（优雅关闭时调用，冲刷未上报的 Trace）
func (t *CozeLoopTracer) Close(ctx context.Context) {
	if t == nil || t.client == nil {
		return
	}
	t.client.Close(ctx)
}
