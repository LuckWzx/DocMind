package agent

// AgentState Agent 执行状态机（规划 3.2.6 ①）
// INIT → Thinking（LLM 推理中）→ Searching（检索/工具执行中）→ Generating（生成中）
//
//	→ Completed / Cancelled / Failed
//
// 状态变化通过 EventState 事件对外推送，前端据此渲染 UI 状态
type AgentState string

const (
	StateInit       AgentState = "init"
	StateThinking   AgentState = "thinking"
	StateSearching  AgentState = "searching"
	StateGenerating AgentState = "generating"
	StateCompleted  AgentState = "completed"
	StateCancelled  AgentState = "cancelled"
	StateFailed     AgentState = "failed"
)

// stateMachine Agent 状态机（单请求内线性推进，非法转换忽略并保持原状态）
type stateMachine struct {
	current AgentState
}

func newStateMachine() *stateMachine {
	return &stateMachine{current: StateInit}
}

// transition 尝试迁移状态，返回是否发生变化
// 允许的转换（环形：Thinking ↔ Searching 多轮工具循环）：
//
//	init       → thinking
//	thinking   → searching / generating / completed / cancelled / failed
//	searching  → thinking / generating / completed / cancelled / failed
//	generating → completed / cancelled / failed
//	任何状态   → cancelled / failed（终态可被中断/错误覆盖）
func (m *stateMachine) transition(next AgentState) bool {
	if m.current == next {
		return false
	}
	if !m.canTransition(next) {
		return false
	}
	m.current = next
	return true
}

// canTransition 校验转换合法性（终态不可再迁移）
func (m *stateMachine) canTransition(next AgentState) bool {
	switch m.current {
	case StateInit:
		return next == StateThinking
	case StateThinking:
		return next == StateSearching || next == StateGenerating ||
			next == StateCompleted || next == StateCancelled || next == StateFailed
	case StateSearching:
		return next == StateThinking || next == StateGenerating ||
			next == StateCompleted || next == StateCancelled || next == StateFailed
	case StateGenerating:
		return next == StateCompleted || next == StateCancelled || next == StateFailed
	default:
		// completed / cancelled / failed 为终态
		return false
	}
}
