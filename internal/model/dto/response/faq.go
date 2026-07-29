package response

import "time"

type FAQResponse struct {
	ID                uint      `json:"id"`
	KnowledgeBaseID   uint      `json:"knowledge_base_id"`
	StandardQuestion  string    `json:"standard_question"`
	Answer            string    `json:"answer"`
	SimilarQuestions  []string  `json:"similar_questions"`
	NegativeQuestions []string  `json:"negative_questions"`
	TagID             *uint     `json:"tag_id"`
	IsEnabled         bool      `json:"is_enabled"`
	IsRecommended     bool      `json:"is_recommended"`
	SortOrder         int       `json:"sort_order"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}
