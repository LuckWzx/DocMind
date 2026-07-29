package response

import "time"

type KnowledgeTagLite struct {
	ID    uint   `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color,omitempty"`
}

type KnowledgeResponse struct {
	ID              uint               `json:"id"`
	KnowledgeBaseID uint               `json:"knowledge_base_id"`
	Title           string             `json:"title"`
	FileName        string             `json:"file_name"`
	Description     string             `json:"description"`
	Type            string             `json:"type"`
	Source          string             `json:"source"`
	Channel         string             `json:"channel"`
	ParseStatus     string             `json:"parse_status"`
	SummaryStatus   string             `json:"summary_status"`
	FileType        string             `json:"file_type"`
	FileSize        int64              `json:"file_size"`
	TagID           uint               `json:"tag_id"`
	Tags            []KnowledgeTagLite `json:"tags,omitempty"`
	ErrorMessage    string             `json:"error_message"`
	CreatedAt       time.Time          `json:"created_at"`
	UpdatedAt       time.Time          `json:"updated_at"`
}
