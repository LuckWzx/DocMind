// 知识库类型定义

export interface KnowledgeBase {
  id: string
  name: string
  description?: string
  type: 'document' | 'faq' | 'wiki'
  created_at: string
  updated_at: string
  document_count?: number
  embedding_model?: string
  vector_store_id?: string
  vector_store_name?: string
  vector_store_engine_type?: string
  vector_store_source?: 'env' | 'user' | 'shared' | 'unavailable'
  vector_store_status?: 'available' | 'unavailable'
  pinned?: boolean
  owner_id?: string
  owner_name?: string
  access_level?: 'owner' | 'admin' | 'contributor' | 'viewer'
}

export interface Knowledge {
  id: string
  knowledge_base_id: string
  title: string
  content?: string
  status: 'pending' | 'processing' | 'completed' | 'failed'
  file_type?: string
  file_size?: number
  source?: string
  created_at: string
  updated_at: string
  tag_ids?: string[]
  tags?: KnowledgeTag[]
}

export interface KnowledgeTag {
  id: string
  name: string
  color?: string
  sort_order?: number
}

export interface CreateKnowledgeBaseParams {
  name: string
  description?: string
  type?: 'document' | 'faq' | 'wiki'
  embedding_model_id?: string
  vector_store_id?: string
}

export interface UpdateKnowledgeBaseParams {
  name: string
  description?: string
  config?: {
    chunking_config?: any
    image_processing_config?: any
  }
}

export interface UploadKnowledgeFileParams {
  file: File
  tag_ids?: string[]
  process_config?: any
}
