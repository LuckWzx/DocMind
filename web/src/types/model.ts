// 模型类型定义

export interface Model {
  id: string
  name: string
  provider: string
  type: 'chat' | 'embedding' | 'rerank'
  enabled: boolean
  config?: ModelConfig
}

export interface ModelConfig {
  api_key?: string
  base_url?: string
  temperature?: number
  max_tokens?: number
}

export interface LLMProvider {
  id: string
  name: string
  type: 'openai' | 'anthropic' | 'deepseek' | 'qwen' | 'zhipu' | 'hunyuan' | 'gemini' | 'minimax' | 'nvidia' | 'ollama'
  enabled: boolean
  models: Model[]
}
