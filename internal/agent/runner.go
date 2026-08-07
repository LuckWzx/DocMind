package agent

import (
	"context"
	"fmt"
	"io"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

const (
	// 步骤参数/结果截断长度（规划风险表：防止工具结果过大撑爆 agent_steps）
	maxArgsLen   = 200
	maxResultLen = 500
)

// EventStream Agent 事件流
// 展开层：消费 adk.AsyncIterator[*adk.AgentEvent] 并转换为项目内统一事件
// （规划 3.2.7：一个 AgentEvent ≠ 一条 SSE，流式消息需逐 chunk 展开）
type EventStream struct {
	ch chan *OutputEvent
}

func newEventStream(ctx context.Context, iter *adk.AsyncIterator[*adk.AgentEvent]) *EventStream {
	s := &EventStream{ch: make(chan *OutputEvent, 64)}
	go s.expand(ctx, iter)
	return s
}

// Next 读取下一个事件；ok=false 表示事件流已结束（含 ctx 取消）
func (s *EventStream) Next() (*OutputEvent, bool) {
	ev, ok := <-s.ch
	return ev, ok
}

// expand 消费 ADK 事件并展开为统一事件
func (s *EventStream) expand(ctx context.Context, iter *adk.AsyncIterator[*adk.AgentEvent]) {
	defer close(s.ch)
	for {
		select {
		case <-ctx.Done():
			return // 调用方取消，直接结束
		default:
		}
		event, ok := iter.Next()
		if !ok {
			s.ch <- &OutputEvent{Type: EventComplete, Done: true}
			return
		}
		// 注意：ADK 的错误在 event.Err 字段，Next 本身不返回 error（规划 3.2.7 ①）
		if event.Err != nil {
			s.ch <- &OutputEvent{Type: EventError, Content: event.Err.Error()}
			return
		}
		// 中断语义（规划 3.2.7 ⑤）：Action.Interrupted → cancelled 状态
		if event.Action != nil && event.Action.Interrupted != nil {
			s.ch <- &OutputEvent{Type: EventState, State: "cancelled"}
			continue
		}
		if event.Output == nil || event.Output.MessageOutput == nil {
			continue
		}
		mv := event.Output.MessageOutput
		switch mv.Role {
		case schema.Assistant:
			s.expandAssistant(ctx, mv)
		case schema.Tool:
			s.expandTool(mv)
		}
	}
}

// expandAssistant 展开模型输出：流式逐 chunk 发 answer 增量；
// 流结束后合并完整消息以提取完整的工具声明（规划 3.2.7 ③ tool_call 增量）
func (s *EventStream) expandAssistant(ctx context.Context, mv *adk.MessageVariant) {
	if !mv.IsStreaming {
		msg, err := mv.GetMessage()
		if err != nil {
			s.ch <- &OutputEvent{Type: EventError, Content: fmt.Sprintf("读取模型输出失败: %v", err)}
			return
		}
		if msg.Content != "" {
			s.ch <- &OutputEvent{Type: EventAnswer, Content: msg.Content}
		}
		s.emitToolDeclarations(msg.ToolCalls)
		return
	}
	// 流式流需独占消费（官方要求 MUST be exclusive），建议开启自动关闭
	mv.MessageStream.SetAutomaticClose()
	chunks := make([]*schema.Message, 0, 8)
	for {
		chunk, err := mv.MessageStream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			s.ch <- &OutputEvent{Type: EventError, Content: fmt.Sprintf("读取流式输出失败: %v", err)}
			return
		}
		if chunk.Content != "" {
			s.ch <- &OutputEvent{Type: EventAnswer, Content: chunk.Content}
		}
		chunks = append(chunks, chunk)
	}
	// 合并增量 chunk，提取完整工具声明（参数跨 chunk 增量拼接）
	full, err := schema.ConcatMessages(chunks)
	if err != nil {
		s.ch <- &OutputEvent{Type: EventError, Content: fmt.Sprintf("合并流式消息失败: %v", err)}
		return
	}
	s.emitToolDeclarations(full.ToolCalls)
}

// expandTool 展开工具结果（Role==Tool）：发 agent_step 事件
func (s *EventStream) expandTool(mv *adk.MessageVariant) {
	msg, err := mv.GetMessage()
	if err != nil {
		s.ch <- &OutputEvent{Type: EventError, Content: fmt.Sprintf("读取工具结果失败: %v", err)}
		return
	}
	result := msg.Content
	if len(result) > maxResultLen {
		result = result[:maxResultLen] + "...(已截断)"
	}
	s.ch <- &OutputEvent{Type: EventStep, ToolName: mv.ToolName, ToolResult: result}
}

// emitToolDeclarations 发送工具调用声明事件（供前端展示步骤开始）
func (s *EventStream) emitToolDeclarations(calls []schema.ToolCall) {
	for _, tc := range calls {
		args := tc.Function.Arguments
		if len(args) > maxArgsLen {
			args = args[:maxArgsLen] + "...(已截断)"
		}
		s.ch <- &OutputEvent{Type: EventStep, ToolName: tc.Function.Name, ToolArgs: args}
	}
}
