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
