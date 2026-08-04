// 聊天类型定义

export interface Session {
  id: string
  title?: string
  knowledge_base_ids?: string[]
  agent_id?: string
  created_at: string
  updated_at: string
  source?: 'web' | 'im' | 'embed'
}

export interface Message {
  id: string
  session_id: string
  role: 'user' | 'assistant' | 'system'
  content: string
  created_at: string
  sources?: MessageSource[]
  tool_calls?: ToolCall[]
  // Agent 执行相关字段（参考 WeKnora 实现）
  agent_steps?: AgentStep[]
  is_completed?: boolean
  agent_duration_ms?: number
  is_fallback?: boolean
}

// Agent 执行步骤（用于持久化 RAG 检索过程）
export interface AgentStep {
  iteration: number
  timestamp: string
  duration?: number
  thought?: string
  reasoning_content?: string
  tool_calls?: AgentStepToolCall[]
}

// Agent 工具调用
export interface AgentStepToolCall {
  id: string
  name: string
  args?: any
  result?: AgentStepToolResult
  duration?: number
}

// Agent 工具调用结果
export interface AgentStepToolResult {
  success: boolean
  output?: string
  error?: string
  data?: any
}

export interface MessageSource {
  id: string
  title: string
  content?: string
  source_type: 'knowledge_base' | 'web'
  score?: number
}

export interface ToolCall {
  id: string
  name: string
  arguments: any
  result?: any
}

export interface SendMessageParams {
  session_id: string
  content: string
  knowledge_base_ids?: string[]
  agent_id?: string
}

export interface StreamMessageParams extends SendMessageParams {
  onMessage?: (chunk: string) => void
  onComplete?: (message: Message) => void
  onError?: (error: Error) => void
}
