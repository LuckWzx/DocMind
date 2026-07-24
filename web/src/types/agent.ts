// Agent类型定义

export interface Agent {
  id: string
  name: string
  description?: string
  avatar?: string
  system_prompt?: string
  model_id?: string
  tools?: AgentTool[]
  created_at: string
  updated_at: string
  is_default?: boolean
}

export interface AgentTool {
  id: string
  name: string
  type: 'builtin' | 'mcp' | 'skill'
  description?: string
  enabled: boolean
}

export interface CreateAgentParams {
  name: string
  description?: string
  system_prompt?: string
  model_id?: string
  tools?: string[]
}

export interface UpdateAgentParams {
  name?: string
  description?: string
  system_prompt?: string
  model_id?: string
  tools?: string[]
}
