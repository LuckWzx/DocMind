package response

import (
	"time"

	"docmind/internal/model/entity"
)

type KnowledgeBaseResponse struct {
	ID                    uint                          `json:"id"`
	Name                  string                        `json:"name"`
	Description           string                        `json:"description"`
	Type                  string                        `json:"type"`
	EmbeddingModelID      string                        `json:"embedding_model_id"`
	SummaryModelID        string                        `json:"summary_model_id"`
	VectorStoreID         *uint                         `json:"vector_store_id"`
	ChunkingConfig        entity.ChunkingConfig         `json:"chunking_config"`
	ExtractConfig         *entity.ExtractConfig         `json:"extract_config,omitempty"`
	FAQConfig             *entity.FAQConfig             `json:"faq_config,omitempty"`
	StorageProviderConfig *entity.StorageProviderConfig `json:"storage_provider_config,omitempty"`
	IndexingStrategy      entity.IndexingStrategy       `json:"indexing_strategy"`
	CreatedAt             time.Time                     `json:"created_at"`
	UpdatedAt             time.Time                     `json:"updated_at"`
	DocumentCount         int64                         `json:"document_count"`
}
