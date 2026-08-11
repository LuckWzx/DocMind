package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"docmind/internal/model/entity"

	"github.com/cloudwego/eino/components/tool"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"

	mcpp "github.com/cloudwego/eino-ext/components/tool/mcp"
)

// defaultTimeout 默认连接/请求超时（秒），未配置高级参数时使用
const defaultTimeout = 10 * time.Second

// Connect 按服务配置创建并初始化 MCP 客户端连接（SSE 需 Start，stdio 已自动启动）
// 返回的连接已 Initialize，可直接 ListTools / CallTool
func Connect(ctx context.Context, svc *entity.MCPService) (client.MCPClient, error) {
	timeout := timeoutOf(svc)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var cli client.MCPClient
	switch svc.TransportType {
	case entity.MCPTransportSSE:
		headers := BuildHeaders(svc)
		c, err := client.NewSSEMCPClient(svc.URL, client.WithHeaders(headers))
		if err != nil {
			return nil, fmt.Errorf("创建 SSE 客户端失败: %w", err)
		}
		// SSE 流必须使用不随本函数返回而取消的 context：defer cancel 会切断长连接，
		// 导致服务端会话被清理、后续请求报 Invalid session ID
		if err := c.Start(context.WithoutCancel(ctx)); err != nil {
			return nil, fmt.Errorf("启动 SSE 连接失败: %w", err)
		}
		cli = c
	case entity.MCPTransportStdio:
		cfg, err := StdioConfig(svc)
		if err != nil {
			return nil, err
		}
		c, err := client.NewStdioMCPClient(cfg.Command, BuildEnv(svc), cfg.Args...)
		if err != nil {
			return nil, fmt.Errorf("启动 stdio 子进程失败: %w", err)
		}
		cli = c
	case entity.MCPTransportHTTPStreamable:
		c, err := client.NewStreamableHttpClient(svc.URL,
			transport.WithHTTPHeaders(BuildHeaders(svc)),
			transport.WithHTTPTimeout(timeoutOf(svc)),
		)
		if err != nil {
			return nil, fmt.Errorf("创建 Streamable HTTP 客户端失败: %w", err)
		}
		// 默认无需常驻连接；调用幂等 Start 以统一初始化语义
		if err := c.Start(ctx); err != nil {
			return nil, fmt.Errorf("启动 Streamable HTTP 连接失败: %w", err)
		}
		cli = c
	default:
		return nil, fmt.Errorf("不支持的传输类型: %s（支持 sse / stdio / http-streamable）", svc.TransportType)
	}

	// 协议握手
	initReq := mcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcp.Implementation{
		Name:    "DocMind",
		Version: "1.0.0",
	}
	if _, err := cli.Initialize(ctx, initReq); err != nil {
		cli.Close()
		return nil, fmt.Errorf("MCP 协议初始化失败: %w", err)
	}
	return cli, nil
}

// BuildHeaders 合并自定义请求头与认证头（api_key / bearer）
func BuildHeaders(svc *entity.MCPService) map[string]string {
	headers := map[string]string{}
	if len(svc.Headers) > 0 {
		var custom map[string]string
		if err := json.Unmarshal(svc.Headers, &custom); err == nil {
			for k, v := range custom {
				headers[k] = v
			}
		}
	}

	auth, err := AuthConfig(svc)
	if err != nil || auth == nil {
		return headers
	}
	switch auth.AuthType {
	case entity.MCPAuthTypeAPIKey:
		if auth.APIKey != "" {
			headerName := auth.APIKeyHeader
			if headerName == "" {
				headerName = "X-API-Key"
			}
			headers[headerName] = auth.APIKey
		}
	case entity.MCPAuthTypeBearer:
		if auth.Token != "" {
			headers["Authorization"] = "Bearer " + auth.Token
		}
	}
	for k, v := range auth.CustomHeaders {
		headers[k] = v
	}
	return headers
}

// BuildEnv 将环境变量配置转为 stdio 子进程的 KEY=VALUE 数组
func BuildEnv(svc *entity.MCPService) []string {
	if len(svc.EnvVars) == 0 {
		return nil
	}
	var env map[string]string
	if err := json.Unmarshal(svc.EnvVars, &env); err != nil {
		return nil
	}
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		out = append(out, k+"="+env[k])
	}
	return out
}

// StdioConfig 解析 stdio 传输配置
func StdioConfig(svc *entity.MCPService) (*entity.MCPServiceStdioConfig, error) {
	var cfg entity.MCPServiceStdioConfig
	if len(svc.StdioConfig) == 0 {
		return nil, fmt.Errorf("stdio 传输缺少 stdio_config 配置")
	}
	if err := json.Unmarshal(svc.StdioConfig, &cfg); err != nil {
		return nil, fmt.Errorf("解析 stdio_config 失败: %w", err)
	}
	if strings.TrimSpace(cfg.Command) == "" {
		return nil, fmt.Errorf("stdio 传输缺少启动命令")
	}
	return &cfg, nil
}

// AuthConfig 解析认证配置
func AuthConfig(svc *entity.MCPService) (*entity.MCPServiceAuthConfig, error) {
	if len(svc.AuthConfig) == 0 {
		return nil, nil
	}
	var cfg entity.MCPServiceAuthConfig
	if err := json.Unmarshal(svc.AuthConfig, &cfg); err != nil {
		return nil, fmt.Errorf("解析 auth_config 失败: %w", err)
	}
	return &cfg, nil
}

// ListTools 拉取 MCP Server 工具列表（转实体元数据，供缓存与展示）
func ListTools(ctx context.Context, cli client.MCPClient, headers map[string]string) ([]entity.MCPServiceTool, error) {
	result, err := cli.ListTools(ctx, mcp.ListToolsRequest{Header: toHeader(headers)})
	if err != nil {
		return nil, fmt.Errorf("拉取工具列表失败: %w", err)
	}
	tools := make([]entity.MCPServiceTool, 0, len(result.Tools))
	for _, t := range result.Tools {
		tools = append(tools, entity.MCPServiceTool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
		})
	}
	return tools, nil
}

// ListResources 拉取 MCP Server 资源列表
func ListResources(ctx context.Context, cli client.MCPClient, headers map[string]string) ([]entity.MCPServiceResource, error) {
	result, err := cli.ListResources(ctx, mcp.ListResourcesRequest{Header: toHeader(headers)})
	if err != nil {
		return nil, fmt.Errorf("拉取资源列表失败: %w", err)
	}
	resources := make([]entity.MCPServiceResource, 0, len(result.Resources))
	for _, res := range result.Resources {
		resources = append(resources, entity.MCPServiceResource{
			URI:         res.URI,
			Name:        res.Name,
			Description: res.Description,
			MimeType:    res.MIMEType,
		})
	}
	return resources, nil
}

// GetEinoTools 将 MCP 工具桥接为 eino tool.BaseTool（可直接注册进 Agent 工具集）
func GetEinoTools(ctx context.Context, cli client.MCPClient, headers map[string]string) ([]tool.BaseTool, error) {
	return mcpp.GetTools(ctx, &mcpp.Config{
		Cli:           cli,
		CustomHeaders: headers,
	})
}

// timeoutOf 获取服务超时配置（秒），未配置时用默认值
func timeoutOf(svc *entity.MCPService) time.Duration {
	if len(svc.AdvancedConfig) == 0 {
		return defaultTimeout
	}
	var cfg entity.MCPServiceAdvancedConfig
	if err := json.Unmarshal(svc.AdvancedConfig, &cfg); err != nil {
		return defaultTimeout
	}
	if cfg.Timeout <= 0 {
		return defaultTimeout
	}
	return time.Duration(cfg.Timeout) * time.Second
}

// toHeader map[string]string → http.Header
func toHeader(headers map[string]string) map[string][]string {
	if len(headers) == 0 {
		return nil
	}
	h := make(map[string][]string, len(headers))
	for k, v := range headers {
		h[k] = []string{v}
	}
	return h
}
