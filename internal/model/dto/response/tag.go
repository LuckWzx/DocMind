package response

import "time"

type TagResponse struct {
	ID              uint      `json:"id"`
	SeqID           uint      `json:"seq_id"`
	Name            string    `json:"name"`
	Color           string    `json:"color"`
	KnowledgeBaseID uint      `json:"knowledge_base_id"`
	SortOrder       int       `json:"sort_order"`
	KnowledgeCount  int64     `json:"knowledge_count"`
	ChunkCount      int64     `json:"chunk_count"`
	FAQCount        int64     `json:"faq_count"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}
