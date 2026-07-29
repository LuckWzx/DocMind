package request

type FAQListRequest struct {
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
	TagID    *uint  `form:"tag_id"`
	Keyword  string `form:"keyword"`
}

type FAQEntryUpsertRequest struct {
	StandardQuestion  string   `json:"standard_question" binding:"required"`
	Answer            string   `json:"answer" binding:"required"`
	SimilarQuestions  []string `json:"similar_questions"`
	NegativeQuestions []string `json:"negative_questions"`
	TagID             *uint    `json:"tag_id"`
	IsEnabled         *bool    `json:"is_enabled"`
	IsRecommended     *bool    `json:"is_recommended"`
}

type FAQEntriesUpsertRequest struct {
	Entries []FAQEntryUpsertRequest `json:"entries" binding:"required"`
	Mode    string                  `json:"mode" binding:"omitempty,oneof=append replace"`
}

type FAQEntryFieldsUpdate struct {
	IsEnabled     *bool `json:"is_enabled"`
	IsRecommended *bool `json:"is_recommended"`
	TagID         *uint `json:"tag_id"`
}

type FAQEntryFieldsBatchRequest struct {
	ByID map[uint]FAQEntryFieldsUpdate `json:"by_id"`
}

type FAQDeleteRequest struct {
	IDs []uint `json:"ids" binding:"required"`
}
