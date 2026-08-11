package request

import "docmind/internal/model/entity"

type CreateKnowledgeBaseRequest struct {
	Name             string      `json:"name" binding:"required,min=1,max=255"`
	Description      string      `json:"description"`
	Type             string      `json:"type" binding:"omitempty,oneof=document faq"`
	EmbeddingModelID string      `json:"embedding_model_id"`
	SummaryModelID   string      `json:"summary_model_id"`
	ChunkingConfig   interface{} `json:"chunking_config"`
	ExtractConfig    interface{} `json:"extract_config"`
	FAQConfig        interface{} `json:"faq_config"`
	IndexingStrategy interface{} `json:"indexing_strategy"`
	VectorStoreID    *uint       `json:"vector_store_id"`
	VLMConfig        interface{} `json:"vlm_config"`
	ASRConfig        interface{} `json:"asr_config"`
}

type UpdateKnowledgeBaseConfigRequest struct {
	ChunkingConfig   interface{} `json:"chunking_config"`
	ExtractConfig    interface{} `json:"extract_config"`
	FAQConfig        interface{} `json:"faq_config"`
	IndexingStrategy interface{} `json:"indexing_strategy"`
}

type UpdateKnowledgeBaseRequest struct {
	Name        string `json:"name" binding:"required,min=1,max=255"`
	Description string `json:"description"`
	// 以下顶层字段与创建接口对齐：编辑知识库时模型/分块/存储配置随基本信息一并更新。
	// 旧客户端仍可走 config.chunking_config 嵌套传参（顶层为空时回退）。
	EmbeddingModelID      string                           `json:"embedding_model_id"`
	SummaryModelID        string                           `json:"summary_model_id"`
	ChunkingConfig        interface{}                      `json:"chunking_config"`
	StorageProviderConfig *entity.StorageProviderConfig    `json:"storage_provider_config"`
	VLMConfig             interface{}                      `json:"vlm_config"`
	ASRConfig             interface{}                      `json:"asr_config"`
	Config                UpdateKnowledgeBaseConfigRequest `json:"config"`
}
