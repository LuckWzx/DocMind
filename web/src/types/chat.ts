// 聊天类型定义

// 后端 sessions.last_request_state JSON 形状（与 stores/settings.ts SessionLastRequestStatePayload 对齐）。
// 字段全部可选——历史会话或新建会话首发前的请求没有这条记录。
export interface SessionLastRequestState {
  agent_id?: string
  agent_enabled?: boolean
  model_id?: string
  knowledge_base_ids?: string[]
  knowledge_ids?: string[]
  tag_ids?: string[]
  mcp_service_ids?: string[]
  skill_names?: string[]
  mentioned_items?: Array<{
    id: string
    name?: string
    type: string
    kb_id?: string
    kb_name?: string
    skill_name?: string
  }>
  web_search_enabled?: boolean
}

export interface Session {
  id: string
  title?: string
  knowledge_base_ids?: string[]
  agent_id?: string
  agent_enabled?: boolean
  last_request_state?: SessionLastRequestState
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
