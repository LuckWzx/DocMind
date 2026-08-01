package request

type KnowledgeListRequest struct {
	Page        int    `form:"page"`
	PageSize    int    `form:"page_size"`
	TagID       string `form:"tag_id"`
	Keyword     string `form:"keyword"`
	FileType    string `form:"file_type"`
	ParseStatus string `form:"parse_status"`
	Source      string `form:"source"`
	StartTime   string `form:"start_time"`
	EndTime     string `form:"end_time"`
}

type ReparseKnowledgeRequest struct {
	ProcessConfig string `json:"process_config"`
}

type KnowledgeTagBatchUpdateRequest struct {
	Updates map[string][]string `json:"updates"`
}
