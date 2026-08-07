package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/adk"
)

// AgentEngine Agent 引擎接口
type AgentEngine interface {
	// Run 启动一次 Agent 运行，返回统一事件流（SSE 层直接消费）
	Run(ctx context.Context, req *RunRequest) (*EventStream, error)
	// RunSync 同步运行，返回最终完整回答（IM 等非流式场景）
	RunSync(ctx context.Context, req *RunRequest) (string, error)
}

// Engine ADK ChatModelAgent 的轻量封装（规划 3.2.1）
type Engine struct {
	agent           adk.Agent
	runner          *adk.Runner
	enableStreaming bool
}

// NewEngine 创建引擎实例
func NewEngine(agent adk.Agent, enableStreaming bool) *Engine {
	return &Engine{
		agent: agent,
		runner: adk.NewRunner(context.Background(), adk.RunnerConfig{
			Agent:           agent,
			EnableStreaming: enableStreaming,
		}),
		enableStreaming: enableStreaming,
	}
}

// Run 启动一次 Agent 运行，返回事件流
func (e *Engine) Run(ctx context.Context, req *RunRequest) (*EventStream, error) {
	if req == nil || len(req.Messages) == 0 {
		return nil, fmt.Errorf("RunRequest.Messages 不能为空")
	}
	// EnableStreaming 需在 AgentInput 显式开启（规划 3.2.7 ④）
	iter := e.runner.Run(ctx, req.Messages)
	return newEventStream(ctx, iter), nil
}

// RunSync 同步运行，拼接流式增量返回完整回答
func (e *Engine) RunSync(ctx context.Context, req *RunRequest) (string, error) {
	stream, err := e.Run(ctx, req)
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	for {
		ev, ok := stream.Next()
		if !ok {
			break
		}
		switch ev.Type {
		case EventAnswer:
			sb.WriteString(ev.Content)
		case EventError:
			return sb.String(), fmt.Errorf("%s", ev.Content)
		}
	}
	return sb.String(), nil
}

var _ AgentEngine = (*Engine)(nil)
