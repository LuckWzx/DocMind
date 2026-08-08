package agent

import (
	"context"
	"fmt"
	"io"
	"time"

	"docmind/internal/model/entity"

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
// 同时自动生成步骤记录（规划 3.2.5：事件流 → entity.AgentSteps）与状态机事件
type EventStream struct {
	ch chan *OutputEvent

	// 步骤记录（expand goroutine 独占写，流结束后由调用方只读）
	steps entity.AgentSteps
	// 状态机（规划 3.2.6 ①：Thinking/Searching/Generating/Completed/Cancelled/Failed）
	sm stateMachine
	// 当前轮次步骤（一次 LLM 推理 = 一轮）
	cur *entity.AgentStep
	// 本轮已声明的工具调用（等待结果填充，含声明时间用于耗时统计）
	pending []*pendingToolCall
}

// pendingToolCall 待填充结果的工具调用（stepIdx 指向所属步骤，供结果/耗时回填）
type pendingToolCall struct {
	call    *entity.AgentStepToolCall
	start   time.Time
	stepIdx int
}

func newEventStream(ctx context.Context, iter *adk.AsyncIterator[*adk.AgentEvent]) *EventStream {
	s := &EventStream{
		ch:    make(chan *OutputEvent, 64),
		steps: entity.AgentSteps{},
		sm:    *newStateMachine(), // 必须显式初始化，零值 state 会拒绝所有转换
	}
	go s.expand(ctx, iter)
	return s
}

// Next 读取下一个事件；ok=false 表示事件流已结束（含 ctx 取消）
func (s *EventStream) Next() (*OutputEvent, bool) {
	ev, ok := <-s.ch
	return ev, ok
}

// Steps 返回本次执行的步骤记录（流结束后调用，用于持久化到 messages.agent_steps）
func (s *EventStream) Steps() entity.AgentSteps {
	return s.steps
}

// emitState 状态变化时推送 state 事件（非法转换静默忽略）
func (s *EventStream) emitState(next AgentState) {
	if !s.sm.transition(next) {
		return
	}
	s.ch <- &OutputEvent{Type: EventState, State: string(next)}
}

// expand 消费 ADK 事件并展开为统一事件
func (s *EventStream) expand(ctx context.Context, iter *adk.AsyncIterator[*adk.AgentEvent]) {
	defer close(s.ch)
	for {
		select {
		case <-ctx.Done():
			return // 调用方取消，直接结束（避免向已无消费者的 channel 写入）
		default:
		}
		event, ok := iter.Next()
		if !ok {
			s.emitState(StateCompleted)
			s.ch <- &OutputEvent{Type: EventComplete, Done: true}
			return
		}
		// 注意：ADK 的错误在 event.Err 字段，Next 本身不返回 error（规划 3.2.7 ①）
		if event.Err != nil {
			s.emitState(StateFailed)
			s.ch <- &OutputEvent{Type: EventError, Content: event.Err.Error()}
			return
		}
		// 中断语义（规划 3.2.7 ⑤）：Action.Interrupted → cancelled 状态
		if event.Action != nil && event.Action.Interrupted != nil {
			s.emitState(StateCancelled)
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
// 每轮 Assistant 事件 = 一条 AgentStep（3.2.5：thinking → Thought，工具 → ToolCalls）
func (s *EventStream) expandAssistant(ctx context.Context, mv *adk.MessageVariant) {
	s.beginStep()
	s.emitState(StateThinking)

	var toolCalls []schema.ToolCall
	if !mv.IsStreaming {
		msg, err := mv.GetMessage()
		if err != nil {
			s.closeStep(len(s.pending) > 0)
			s.ch <- &OutputEvent{Type: EventError, Content: fmt.Sprintf("读取模型输出失败: %v", err)}
			return
		}
		if msg.Content != "" {
			s.ch <- &OutputEvent{Type: EventAnswer, Content: msg.Content}
			s.cur.Thought += msg.Content
		}
		toolCalls = msg.ToolCalls
	} else {
		// 流式流需独占消费（官方要求 MUST be exclusive），建议开启自动关闭
		mv.MessageStream.SetAutomaticClose()
		chunks := make([]*schema.Message, 0, 8)
		for {
			chunk, err := mv.MessageStream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				s.closeStep(len(s.pending) > 0)
				s.ch <- &OutputEvent{Type: EventError, Content: fmt.Sprintf("读取流式输出失败: %v", err)}
				return
			}
			if chunk.Content != "" {
				s.ch <- &OutputEvent{Type: EventAnswer, Content: chunk.Content}
				s.cur.Thought += chunk.Content
			}
			chunks = append(chunks, chunk)
		}
		// 合并增量 chunk，提取完整工具声明（参数跨 chunk 增量拼接）
		full, err := schema.ConcatMessages(chunks)
		if err != nil {
			s.closeStep(len(s.pending) > 0)
			s.ch <- &OutputEvent{Type: EventError, Content: fmt.Sprintf("合并流式消息失败: %v", err)}
			return
		}
		toolCalls = full.ToolCalls
	}

	// 收尾本轮：有工具调用 → 声明 + 进入 searching；纯回答 → 进入 generating
	s.emitToolDeclarations(toolCalls)
	hasTools := len(toolCalls) > 0
	s.closeStep(hasTools)
	if hasTools {
		s.emitState(StateSearching)
	} else {
		s.emitState(StateGenerating)
	}
}

// expandTool 展开工具结果（Role==Tool）：发 agent_step 事件 + 填充步骤记录
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
	s.fillToolResult(mv.ToolName, result)
}

// beginStep 开启新一轮步骤（上一轮未收尾时先收尾；清理上一轮遗留的工具声明）
func (s *EventStream) beginStep() {
	if s.cur != nil {
		s.closeStep(false)
	}
	// 上一轮的工具结果应在新一轮 Assistant 事件前全部到达，遗留即丢弃
	s.pending = nil
	s.cur = &entity.AgentStep{
		Iteration: len(s.steps) + 1,
		Timestamp: time.Now(),
	}
}

// closeStep 收尾当前轮次：有工具调用则挂载已声明的工具调用（结果/耗时由
// fillToolResult 在工具结果到达时同步回填）；纯回答轮 Thought 即答案正文，不重复记录
func (s *EventStream) closeStep(hasTools bool) {
	if s.cur == nil {
		return
	}
	s.cur.Duration = time.Since(s.cur.Timestamp).Milliseconds()
	if hasTools {
		stepIdx := len(s.steps)
		calls := make([]entity.AgentStepToolCall, 0, len(s.pending))
		for _, p := range s.pending {
			p.stepIdx = stepIdx
			calls = append(calls, *p.call)
		}
		s.cur.ToolCalls = calls
	} else {
		s.cur.Thought = ""
	}
	s.steps = append(s.steps, *s.cur)
	s.cur = nil
}

// emitToolDeclarations 发送工具调用声明事件 + 登记步骤记录（参数用完整原始值，SSE 用截断）
func (s *EventStream) emitToolDeclarations(calls []schema.ToolCall) {
	for _, tc := range calls {
		args := tc.Function.Arguments
		if len(args) > maxArgsLen {
			args = args[:maxArgsLen] + "...(已截断)"
		}
		s.ch <- &OutputEvent{Type: EventStep, ToolName: tc.Function.Name, ToolArgs: args}
		s.pending = append(s.pending, &pendingToolCall{
			call: &entity.AgentStepToolCall{
				ID:   tc.ID,
				Name: tc.Function.Name,
				Args: tc.Function.Arguments,
			},
			start: time.Now(),
		})
	}
}

// fillToolResult 将工具结果回填到所属步骤的同名未完成工具调用（耗时=声明→结果）
func (s *EventStream) fillToolResult(toolName, result string) {
	for i, p := range s.pending {
		if p.call.Name != toolName {
			continue
		}
		duration := time.Since(p.start).Milliseconds()
		p.call.Duration = duration
		// 同步回填步骤中的拷贝（同轮多个同名工具时填第一个未完成的）
		if p.stepIdx >= 0 && p.stepIdx < len(s.steps) {
			step := &s.steps[p.stepIdx]
			for j := range step.ToolCalls {
				if step.ToolCalls[j].Name == toolName && step.ToolCalls[j].Result == nil {
					step.ToolCalls[j].Duration = duration
					step.ToolCalls[j].Result = &entity.AgentStepToolResult{Success: true, Output: result}
					break
				}
			}
		}
		s.pending = append(s.pending[:i], s.pending[i+1:]...)
		return
	}
}
