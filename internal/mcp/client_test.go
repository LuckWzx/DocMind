package mcp

import (
	"context"
	"strings"
	"testing"
	"time"

	"docmind/internal/model/entity"

	"github.com/cloudwego/eino/components/tool"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// TestStdioConnect 验证 stdio 传输全链路：连接 → 初始化 → 工具列表 → eino 桥接 → 工具调用
// 依赖 scripts/mcp_test_server.py（纯标准库，无外部依赖）
func TestStdioConnect(t *testing.T) {
	svc := &entity.MCPService{
		TransportType: entity.MCPTransportStdio,
		StdioConfig:   entity.JSON(`{"command":"python","args":["../../scripts/mcp_test_server.py"]}`),
	}
	cli, err := Connect(context.Background(), svc)
	if err != nil {
		t.Fatalf("Connect 失败: %v", err)
	}
	defer cli.Close()

	tools, err := ListTools(context.Background(), cli, nil)
	if err != nil {
		t.Fatalf("ListTools 失败: %v", err)
	}
	if len(tools) != 2 {
		t.Fatalf("期望 2 个工具，实际 %d 个: %+v", len(tools), tools)
	}

	einoTools, err := GetEinoTools(context.Background(), cli, nil)
	if err != nil {
		t.Fatalf("GetEinoTools 失败: %v", err)
	}
	if len(einoTools) != 2 {
		t.Fatalf("期望 2 个 eino 工具，实际 %d 个", len(einoTools))
	}

	// 找到 add_numbers 工具并执行
	var target tool.InvokableTool
	for _, bt := range einoTools {
		info, err := bt.Info(context.Background())
		if err != nil {
			t.Fatalf("Info 失败: %v", err)
		}
		if info.Name == "add_numbers" {
			target = bt.(tool.InvokableTool)
		}
	}
	if target == nil {
		t.Fatalf("未找到 add_numbers 工具")
	}
	result, err := target.InvokableRun(context.Background(), `{"a": 3, "b": 4}`)
	if err != nil {
		t.Fatalf("InvokableRun 失败: %v", err)
	}
	// eino 桥接返回完整 CallToolResult JSON（含 content 数组），断言结果文本包含求和值
	if !contains(result, "sum") || !contains(result, "7") {
		t.Fatalf("期望 sum=7，实际结果: %s", result)
	}
}

// TestSSEConnect 验证 SSE 传输全链路（本地起 mcp-go SSE Server）
func TestSSEConnect(t *testing.T) {
	const addr = "127.0.0.1:18999"
	svr := server.NewMCPServer("test-sse", mcp.LATEST_PROTOCOL_VERSION)
	svr.AddTool(mcp.NewTool("echo_text",
		mcp.WithDescription("原样返回输入文本"),
		mcp.WithString("text", mcp.Required()),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		text, _ := req.Params.Arguments.(map[string]any)["text"].(string)
		return mcp.NewToolResultText(text), nil
	})
	go func() {
		if err := server.NewSSEServer(svr, server.WithBaseURL("http://"+addr)).Start(addr); err != nil {
			t.Errorf("SSE Server 启动失败: %v", err)
		}
	}()
	time.Sleep(500 * time.Millisecond)

	svc := &entity.MCPService{
		TransportType: entity.MCPTransportSSE,
		URL:           "http://" + addr + "/sse",
	}
	cli, err := Connect(context.Background(), svc)
	if err != nil {
		t.Fatalf("Connect 失败: %v", err)
	}
	defer cli.Close()

	tools, err := ListTools(context.Background(), cli, nil)
	if err != nil {
		t.Fatalf("ListTools 失败: %v", err)
	}
	if len(tools) != 1 || tools[0].Name != "echo_text" {
		t.Fatalf("期望 1 个 echo_text 工具，实际: %+v", tools)
	}

	einoTools, err := GetEinoTools(context.Background(), cli, nil)
	if err != nil {
		t.Fatalf("GetEinoTools 失败: %v", err)
	}
	inv := einoTools[0].(tool.InvokableTool)
	result, err := inv.InvokableRun(context.Background(), `{"text": "hello mcp"}`)
	if err != nil {
		t.Fatalf("InvokableRun 失败: %v", err)
	}
	if !contains(result, "hello mcp") {
		t.Fatalf("期望回显 hello mcp，实际结果: %s", result)
	}
}

// TestHTTPStreamableConnect 验证 Streamable HTTP 传输全链路（新版远程规范 2025-03-26）
func TestHTTPStreamableConnect(t *testing.T) {
	const addr = "127.0.0.1:18998"
	svr := server.NewMCPServer("test-http", mcp.LATEST_PROTOCOL_VERSION)
	svr.AddTool(mcp.NewTool("echo_text",
		mcp.WithDescription("原样返回输入文本"),
		mcp.WithString("text", mcp.Required()),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		text, _ := req.Params.Arguments.(map[string]any)["text"].(string)
		return mcp.NewToolResultText(text), nil
	})
	go func() {
		if err := server.NewStreamableHTTPServer(svr).Start(addr); err != nil {
			t.Errorf("Streamable HTTP Server 启动失败: %v", err)
		}
	}()
	time.Sleep(500 * time.Millisecond)

	svc := &entity.MCPService{
		TransportType: entity.MCPTransportHTTPStreamable,
		URL:           "http://" + addr + "/mcp",
	}
	cli, err := Connect(context.Background(), svc)
	if err != nil {
		t.Fatalf("Connect 失败: %v", err)
	}
	defer cli.Close()

	tools, err := ListTools(context.Background(), cli, nil)
	if err != nil {
		t.Fatalf("ListTools 失败: %v", err)
	}
	if len(tools) != 1 || tools[0].Name != "echo_text" {
		t.Fatalf("期望 1 个 echo_text 工具，实际: %+v", tools)
	}

	einoTools, err := GetEinoTools(context.Background(), cli, nil)
	if err != nil {
		t.Fatalf("GetEinoTools 失败: %v", err)
	}
	inv := einoTools[0].(tool.InvokableTool)
	result, err := inv.InvokableRun(context.Background(), `{"text": "hello http"}`)
	if err != nil {
		t.Fatalf("InvokableRun 失败: %v", err)
	}
	if !contains(result, "hello http") {
		t.Fatalf("期望回显 hello http，实际结果: %s", result)
	}
}

func contains(s, sub string) bool {
	return strings.Contains(s, sub)
}
