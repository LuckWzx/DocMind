package request

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
}

type UpdateKnowledgeBaseConfigRequest struct {
	ChunkingConfig   interface{} `json:"chunking_config"`
	ExtractConfig    interface{} `json:"extract_config"`
	FAQConfig        interface{} `json:"faq_config"`
	IndexingStrategy interface{} `json:"indexing_strategy"`
}

type UpdateKnowledgeBaseRequest struct {
	Name        string                           `json:"name" binding:"required,min=1,max=255"`
	Description string                           `json:"description"`
	Config      UpdateKnowledgeBaseConfigRequest `json:"config"`
}
